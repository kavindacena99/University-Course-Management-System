import { FormEvent, useEffect, useState } from "react";
import { Link, useNavigate } from "react-router-dom";
import type { AccountProfile, UserSession } from "../types/api";
import {
  changeAccountPassword,
  getAccountProfile,
  listAccountSessions,
  logoutAllSessions,
  revokeAccountSession,
  updateAccountProfile,
} from "../services/account";
import { endSession } from "../services/api";
import "./AccountSettings.css";

export default function AccountSettings() {
  const navigate = useNavigate();
  const [profile, setProfile] = useState<AccountProfile | null>(null);
  const [sessions, setSessions] = useState<UserSession[]>([]);
  const [firstName, setFirstName] = useState("");
  const [lastName, setLastName] = useState("");
  const [currentPassword, setCurrentPassword] = useState("");
  const [newPassword, setNewPassword] = useState("");
  const [confirmPassword, setConfirmPassword] = useState("");
  const [loading, setLoading] = useState(true);
  const [savingProfile, setSavingProfile] = useState(false);
  const [savingPassword, setSavingPassword] = useState(false);
  const [busySession, setBusySession] = useState("");
  const [error, setError] = useState("");
  const [notice, setNotice] = useState("");

  async function refreshSessions() {
    const response = await listAccountSessions();
    setSessions(response.sessions);
  }

  useEffect(() => {
    let active = true;
    Promise.all([getAccountProfile(), listAccountSessions()])
      .then(([profileResponse, sessionsResponse]) => {
        if (!active) return;
        setProfile(profileResponse.user);
        setFirstName(profileResponse.user.first_name);
        setLastName(profileResponse.user.last_name);
        setSessions(sessionsResponse.sessions);
      })
      .catch((requestError: unknown) => {
        if (active) setError(requestError instanceof Error ? requestError.message : "Unable to load account settings.");
      })
      .finally(() => {
        if (active) setLoading(false);
      });
    return () => {
      active = false;
    };
  }, []);

  async function saveProfile(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!profile) return;
    setSavingProfile(true);
    setError("");
    setNotice("");
    try {
      const response = await updateAccountProfile({
        version: profile.version,
        first_name: firstName.trim(),
        last_name: lastName.trim(),
      });
      setProfile(response.user);
      setFirstName(response.user.first_name);
      setLastName(response.user.last_name);
      setNotice("Profile updated.");
    } catch (requestError) {
      setError(requestError instanceof Error ? requestError.message : "Unable to update your profile.");
    } finally {
      setSavingProfile(false);
    }
  }

  async function savePassword(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (newPassword !== confirmPassword) {
      setError("New password and confirmation do not match.");
      return;
    }
    setSavingPassword(true);
    setError("");
    setNotice("");
    try {
      await changeAccountPassword(currentPassword, newPassword);
      setCurrentPassword("");
      setNewPassword("");
      setConfirmPassword("");
      setNotice("Password changed successfully.");
    } catch (requestError) {
      setError(requestError instanceof Error ? requestError.message : "Unable to change your password.");
    } finally {
      setSavingPassword(false);
    }
  }

  async function revokeSession(session: UserSession) {
    setBusySession(session.id);
    setError("");
    setNotice("");
    try {
      await revokeAccountSession(session.id);
      await refreshSessions();
      setNotice("Session revoked.");
    } catch (requestError) {
      setError(requestError instanceof Error ? requestError.message : "Unable to revoke this session.");
    } finally {
      setBusySession("");
    }
  }

  async function signOutEverywhere() {
    setError("");
    try {
      await logoutAllSessions();
      sessionStorage.clear();
      endSession();
      navigate("/login", { replace: true });
    } catch (requestError) {
      setError(requestError instanceof Error ? requestError.message : "Unable to sign out all sessions.");
    }
  }

  return (
    <main className="account-settings">
      <header className="account-settings__header">
        <div>
          <p>Account</p>
          <h1>Profile and security</h1>
        </div>
        <Link to="/courses">Back to courses</Link>
      </header>

      {error && <p className="account-settings__message account-settings__message--error" role="alert">{error}</p>}
      {notice && <p className="account-settings__message" role="status">{notice}</p>}
      {loading ? (
        <p role="status">Loading account settings...</p>
      ) : profile ? (
        <>
          <section>
            <h2>Profile</h2>
            <p className="account-settings__email">{profile.email}</p>
            <form onSubmit={saveProfile}>
              <label>
                First name
                <input maxLength={100} onChange={(event) => setFirstName(event.target.value)} required value={firstName} />
              </label>
              <label>
                Last name
                <input maxLength={100} onChange={(event) => setLastName(event.target.value)} required value={lastName} />
              </label>
              <button disabled={savingProfile} type="submit">{savingProfile ? "Saving..." : "Save profile"}</button>
            </form>
          </section>

          <section>
            <h2>Change password</h2>
            <form onSubmit={savePassword}>
              <label>
                Current password
                <input autoComplete="current-password" onChange={(event) => setCurrentPassword(event.target.value)} required type="password" value={currentPassword} />
              </label>
              <label>
                New password
                <input autoComplete="new-password" onChange={(event) => setNewPassword(event.target.value)} required type="password" value={newPassword} />
              </label>
              <label>
                Confirm new password
                <input autoComplete="new-password" onChange={(event) => setConfirmPassword(event.target.value)} required type="password" value={confirmPassword} />
              </label>
              <button disabled={savingPassword} type="submit">{savingPassword ? "Changing..." : "Change password"}</button>
            </form>
          </section>

          <section>
            <div className="account-settings__section-heading">
              <div>
                <h2>Active sessions</h2>
                <p>Revoke sessions you no longer recognize or use.</p>
              </div>
              <button className="account-settings__danger" onClick={() => void signOutEverywhere()} type="button">
                Sign out all devices
              </button>
            </div>
            {sessions.length ? (
              <div className="account-settings__sessions">
                {sessions.map((session) => (
                  <article key={session.id}>
                    <div>
                      <strong>{session.current ? "This device" : session.user_agent || "Other session"}</strong>
                      <span>Last used {new Date(session.last_used_at).toLocaleString()}</span>
                      {session.ip_address && <span>{session.ip_address}</span>}
                    </div>
                    {session.current ? <span className="account-settings__current">Current</span> : (
                      <button
                        disabled={busySession === session.id}
                        onClick={() => void revokeSession(session)}
                        type="button"
                      >
                        {busySession === session.id ? "Revoking..." : "Revoke"}
                      </button>
                    )}
                  </article>
                ))}
              </div>
            ) : <p>No active sessions.</p>}
          </section>
        </>
      ) : null}
    </main>
  );
}
