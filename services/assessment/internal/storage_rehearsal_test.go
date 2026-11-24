package assessment

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"assessment/internal/config"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/minio/minio-go/v7/pkg/signer"
)

// This opt-in integration test is executed only on two new synthetic storage
// projects. It never uses the application's ordinary endpoint or credentials.
func TestObjectStorageRehearsal(t *testing.T) {
	if os.Getenv("STORAGE_REHEARSAL") != "isolated-synthetic-only" {
		t.Skip("explicit isolated storage rehearsal is not configured")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	client := func(endpoint, user, password string) *minio.Client {
		t.Helper()
		c, err := minio.New(endpoint, &minio.Options{Creds: credentials.NewStaticV4(user, password, ""), Region: "us-east-1"})
		if err != nil {
			t.Fatal(err)
		}
		return c
	}
	const source = "platform-storage-remediation-20261123-minio-1:9000"
	const restore = "platform-storage-restore-remediation-20261123-minio-1:9000"
	rootUser, rootPassword := os.Getenv("STORAGE_ROOT_USER"), os.Getenv("STORAGE_ROOT_PASSWORD")
	appUser, appPassword := os.Getenv("STORAGE_APP_USER"), os.Getenv("STORAGE_APP_PASSWORD")
	admin := client(source, rootUser, rootPassword)
	app := client(source, appUser, appPassword)
	destination := client(restore, os.Getenv("RESTORE_ROOT_USER"), os.Getenv("RESTORE_ROOT_PASSWORD"))
	destinationApp := client(restore, os.Getenv("RESTORE_APP_USER"), os.Getenv("RESTORE_APP_PASSWORD"))
	prefix := "rehearsal-" + time.Now().UTC().Format("20060102T150405.000000000") + "/"
	store, err := NewMinIOStore(&config.Config{ObjectStorageEndpoint: source,
		ObjectStorageAccessKey: appUser, ObjectStorageSecretKey: appPassword, ObjectStorageBucket: "submissions"})
	if err != nil {
		t.Fatal(err)
	}
	if err = store.EnsureBucket(ctx); err != nil {
		t.Fatal(err)
	}
	if err = store.Ready(ctx); err != nil {
		t.Fatal(err)
	}
	key := prefix + "normal.csv"
	data := []byte("course,score\nCS101,80\n") // Tiny safe input, never an exhaustion payload.
	if err = store.Put(ctx, key, "text/csv", int64(len(data)), bytes.NewReader(data)); err != nil {
		t.Fatal(err)
	}
	reader, contentType, err := store.Get(ctx, key)
	if err != nil {
		t.Fatal(err)
	}
	got, err := io.ReadAll(reader)
	_ = reader.Close()
	if err != nil || !bytes.Equal(got, data) || contentType != "text/csv" {
		t.Fatal("application Put/Get/Stat mismatch")
	}
	large := bytes.Repeat([]byte("synthetic multipart data\n"), 300000)
	largeKey := prefix + "multipart.bin"
	if _, err = app.PutObject(ctx, "submissions", largeKey, bytes.NewReader(large), int64(len(large)),
		minio.PutObjectOptions{PartSize: 5 << 20, ContentType: "application/octet-stream", UserMetadata: map[string]string{"Rehearsal": "synthetic"}}); err != nil {
		t.Fatal(err)
	}
	link, err := app.PresignedGetObject(ctx, "submissions", key, time.Minute, url.Values{})
	if err != nil {
		t.Fatal(err)
	}
	response, err := http.Get(link.String())
	if err != nil {
		t.Fatal("presigned request failed")
	}
	got, err = io.ReadAll(response.Body)
	_ = response.Body.Close()
	if err != nil || response.StatusCode != 200 || !bytes.Equal(got, data) {
		t.Fatal("presigned GET mismatch")
	}
	for _, c := range []*minio.Client{app, client(source, "invalid-rehearsal-user", "invalid-rehearsal-password")} {
		if err = c.MakeBucket(ctx, "forbidden-rehearsal", minio.MakeBucketOptions{}); minio.ToErrorResponse(err).StatusCode != 403 {
			t.Fatalf("unprivileged bucket creation: %v", err)
		}
	}
	if err = admin.MakeBucket(ctx, "private-rehearsal", minio.MakeBucketOptions{}); err != nil {
		exists, check := admin.BucketExists(ctx, "private-rehearsal")
		if check != nil || !exists {
			t.Fatal(err)
		}
	}
	if _, err = admin.PutObject(ctx, "private-rehearsal", "private", strings.NewReader("synthetic private data"), 22, minio.PutObjectOptions{}); err != nil {
		t.Fatal(err)
	}
	// Silo's ListBucketsHandler intentionally filters a denied ListAllMyBuckets
	// request by ListBucket/GetBucketLocation. Verify the actual visibility boundary.
	buckets, err := app.ListBuckets(ctx)
	if err != nil || len(buckets) != 1 || buckets[0].Name != "submissions" {
		t.Fatalf("application bucket visibility escaped its policy: %v %v", buckets, err)
	}
	if _, err = app.StatObject(ctx, "private-rehearsal", "private", minio.StatObjectOptions{}); minio.ToErrorResponse(err).StatusCode != 403 {
		t.Fatalf("cross-bucket access not denied: %v", err)
	}
	if _, err = app.PutObject(ctx, "private-rehearsal", "private", strings.NewReader("blocked"), 7, minio.PutObjectOptions{}); minio.ToErrorResponse(err).StatusCode != 403 {
		t.Fatalf("cross-bucket write not denied: %v", err)
	}
	if err = app.RemoveObject(ctx, "private-rehearsal", "private", minio.RemoveObjectOptions{}); minio.ToErrorResponse(err).StatusCode != 403 {
		t.Fatalf("cross-bucket deletion not denied: %v", err)
	}
	if _, err = admin.StatObject(ctx, "private-rehearsal", "private", minio.StatObjectOptions{}); err != nil {
		t.Fatal("denied application requests changed private fixture")
	}
	response, err = http.Get("http://" + source + "/submissions/" + key)
	if err != nil {
		t.Fatal(err)
	}
	_ = response.Body.Close()
	if response.StatusCode != 403 {
		t.Fatal("anonymous read was not forbidden")
	}

	queries := []string{"select&select-type=2", "select=&select-type=2", "select-type=2&select",
		"%73elect&select-type=2", "s%65lect&select-type=2", "se%6cect&select-type=2",
		"%73%65%6c%65%63%74&select-type=2", "SeLeCt&select-type=2", "select=anything&select-type=2"}
	for _, target := range []struct{ endpoint, rootUser, rootPassword, appUser, appPassword string }{
		{source, rootUser, rootPassword, appUser, appPassword},
		{restore, os.Getenv("RESTORE_ROOT_USER"), os.Getenv("RESTORE_ROOT_PASSWORD"), os.Getenv("RESTORE_APP_USER"), os.Getenv("RESTORE_APP_PASSWORD")},
	} {
		for _, identity := range [][2]string{{target.rootUser, target.rootPassword}, {target.appUser, target.appPassword}, {"", ""}} {
			for _, query := range queries {
				request, err := http.NewRequestWithContext(ctx, http.MethodPost, "http://"+target.endpoint+"/submissions/"+key+"?"+query,
					strings.NewReader(`<SelectObjectContentRequest><Expression>SELECT * FROM S3Object</Expression><ExpressionType>SQL</ExpressionType><InputSerialization><CSV><FileHeaderInfo>USE</FileHeaderInfo></CSV></InputSerialization><OutputSerialization><CSV/></OutputSerialization></SelectObjectContentRequest>`))
				if err != nil {
					t.Fatal(err)
				}
				if identity[0] != "" {
					request = signer.SignV4(*request, identity[0], identity[1], "", "us-east-1")
				}
				request.URL.RawQuery = query // Keep encoded-key variants on the wire after canonical signing.
				result, err := http.DefaultClient.Do(request)
				if err != nil {
					t.Fatal("safe Select policy request failed")
				}
				_ = result.Body.Close()
				if result.StatusCode != 403 || result.Header.Get("X-S3-Select-Guard") != "denied" {
					t.Fatalf("Select bypass for query %q: status %d", query, result.StatusCode)
				}
			}
		}
	}
	for _, endpoint := range []string{strings.Replace(source, ":9000", ":19000", 1), strings.Replace(restore, ":9000", ":19000", 1),
		strings.Replace(source, ":9000", ":19001", 1), strings.Replace(restore, ":9000", ":19001", 1)} {
		connection, err := net.DialTimeout("tcp", endpoint, 2*time.Second)
		if err == nil {
			_ = connection.Close()
			t.Fatal("unguarded Silo listener is reachable from client network")
		}
	}

	backupDir := os.Getenv("STORAGE_BACKUP_DIR")
	if backupDir == "" {
		t.Fatal("protected synthetic backup directory is required")
	}
	if err = os.Mkdir(backupDir, 0700); err != nil {
		t.Fatal(err)
	}
	manifest := map[string]map[string]string{}
	for object := range app.ListObjects(ctx, "submissions", minio.ListObjectsOptions{Prefix: prefix, Recursive: true}) {
		if object.Err != nil {
			t.Fatal(object.Err)
		}
		objectReader, err := app.GetObject(ctx, "submissions", object.Key, minio.GetObjectOptions{})
		if err != nil {
			t.Fatal(err)
		}
		info, err := objectReader.Stat()
		if err != nil {
			t.Fatal(err)
		}
		body, err := io.ReadAll(objectReader)
		_ = objectReader.Close()
		if err != nil {
			t.Fatal(err)
		}
		sum := sha256.Sum256(body)
		hash := hex.EncodeToString(sum[:])
		if err = os.WriteFile(filepath.Join(backupDir, hash), body, 0600); err != nil {
			t.Fatal(err)
		}
		manifest[object.Key] = map[string]string{"sha256": hash, "content_type": info.ContentType, "rehearsal_metadata": info.Metadata.Get("X-Amz-Meta-Rehearsal")}
		if _, err = destination.PutObject(ctx, "submissions", object.Key, bytes.NewReader(body), int64(len(body)),
			minio.PutObjectOptions{ContentType: info.ContentType, UserMetadata: info.UserMetadata}); err != nil {
			t.Fatal(err)
		}
		restored, err := destinationApp.GetObject(ctx, "submissions", object.Key, minio.GetObjectOptions{})
		if err != nil {
			t.Fatal(err)
		}
		restoredInfo, err := restored.Stat()
		if err != nil {
			t.Fatal(err)
		}
		restoredBody, err := io.ReadAll(restored)
		_ = restored.Close()
		if err != nil || sha256.Sum256(restoredBody) != sum || restoredInfo.ContentType != info.ContentType ||
			restoredInfo.Metadata.Get("X-Amz-Meta-Rehearsal") != info.Metadata.Get("X-Amz-Meta-Rehearsal") {
			t.Fatal("restored content/metadata mismatch")
		}
	}
	if len(manifest) != 2 {
		t.Fatalf("expected two disposable objects, got %d", len(manifest))
	}
	encoded, _ := json.MarshalIndent(manifest, "", "  ")
	if err = os.WriteFile(filepath.Join(backupDir, "manifest.json"), encoded, 0600); err != nil {
		t.Fatal(err)
	}
	if err = store.Delete(ctx, key); err != nil {
		t.Fatal(err)
	}
	if _, err = app.StatObject(ctx, "submissions", key, minio.StatObjectOptions{}); minio.ToErrorResponse(err).StatusCode != 404 {
		t.Fatal("disposable delete failed")
	}
	t.Log("PASS application SDK, multipart, presigned GET, IAM/anonymous boundaries, 54 guarded Select variants, direct-port denial, two-object checksum/metadata restore")
}
