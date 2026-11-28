import { ChangeEvent, useCallback, useEffect, useState } from "react";
import { Link, useSearchParams } from "react-router-dom";
import type { CourseOffering, FinalResult } from "../types/api";
import { getOffering, importFinalResults, listFinalResults } from "../services/assessments";
import "./AdminFinalResults.css";

export default function AdminFinalResults() {
  const [searchParams] = useSearchParams();
  const offeringId = searchParams.get("offeringId") ?? "";
  const [offering, setOffering] = useState<CourseOffering | null>(null);
  const [results, setResults] = useState<FinalResult[]>([]);
  const [file, setFile] = useState<File | null>(null);
  const [loading, setLoading] = useState(true);
  const [importing, setImporting] = useState(false);
  const [error, setError] = useState("");
  const [notice, setNotice] = useState("");

  const refresh = useCallback(async (id: string) => {
    const [offeringResponse, resultResponse] = await Promise.all([
      getOffering(id),
      listFinalResults(id),
    ]);
    setOffering(offeringResponse.offering);
    setResults(resultResponse.results);
  }, []);

  useEffect(() => {
    if (!offeringId) {
      setError("Select a course offering before managing final results.");
      setLoading(false);
      return;
    }
    let active = true;
    refresh(offeringId)
      .catch((requestError: unknown) => {
        if (active) setError(requestError instanceof Error ? requestError.message : "Unable to load final results.");
      })
      .finally(() => {
        if (active) setLoading(false);
      });
    return () => {
      active = false;
    };
  }, [offeringId, refresh]);

  function chooseFile(event: ChangeEvent<HTMLInputElement>) {
    setFile(event.currentTarget.files?.[0] ?? null);
    setError("");
    setNotice("");
  }

  async function submitImport() {
    if (!offeringId || !file) {
      setError("Choose a CSV file to import.");
      return;
    }
    if (!file.name.toLowerCase().endsWith(".csv")) {
      setError("Choose a file with the .csv extension.");
      return;
    }
    setImporting(true);
    setError("");
    setNotice("");
    try {
      const response = await importFinalResults(offeringId, file);
      await refresh(offeringId);
      setFile(null);
      setNotice(
        response.import.already_processed
          ? "This import was already processed."
          : `Imported ${response.import.row_count} final result(s).`,
      );
    } catch (requestError) {
      setError(requestError instanceof Error ? requestError.message : "Unable to import final results.");
    } finally {
      setImporting(false);
    }
  }

  return (
    <main className="admin-final-results">
      <header>
        <div>
          <p>Administration</p>
          <h1>Final results</h1>
          <span>
            {offering
              ? `${offering.course_code} - ${offering.course_title} · ${offering.academic_year} semester ${offering.semester}`
              : ""}
          </span>
        </div>
        <Link to={offering ? `/lecturer/courses/view?courseId=${encodeURIComponent(offering.course_id)}` : "/admin"}>
          Back
        </Link>
      </header>

      {error && <p className="admin-final-results__message admin-final-results__message--error" role="alert">{error}</p>}
      {notice && <p className="admin-final-results__message" role="status">{notice}</p>}
      {loading ? (
        <p role="status">Loading final results...</p>
      ) : (
        <>
          <section>
            <h2>Import final results</h2>
            <p>CSV columns must be exactly student_user_id,final_mark,max_mark. Upload 1–1,000 rows, up to 5 MiB.</p>
            <label>
              CSV file
              <input accept=".csv,text/csv" onChange={chooseFile} type="file" />
            </label>
            {file && <span className="admin-final-results__filename">{file.name}</span>}
            <button disabled={importing || !file} onClick={() => void submitImport()} type="button">
              {importing ? "Importing..." : "Import results"}
            </button>
          </section>

          <section>
            <h2>Imported results</h2>
            {results.length ? (
              <div className="admin-final-results__table-wrap">
                <table>
                  <thead><tr><th>Student ID</th><th>Final mark</th><th>Maximum</th><th>Imported</th></tr></thead>
                  <tbody>
                    {results.map((result) => (
                      <tr key={result.id}>
                        <td>{result.student_user_id}</td>
                        <td>{result.final_mark}</td>
                        <td>{result.max_mark}</td>
                        <td>{new Date(result.imported_at).toLocaleString()}</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            ) : <p>No final results have been imported for this offering.</p>}
          </section>
        </>
      )}
    </main>
  );
}
