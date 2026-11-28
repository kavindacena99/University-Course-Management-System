import { FormEvent, useState } from "react";
import { useNavigate } from "react-router-dom";
import { useSession } from "../session";

const assetPathPrefix = "/assets";

type Notice = {
  tone: "success" | "info" | "error";
  text: string;
} | null;

export default function Login() {
  const [notice, setNotice] = useState<Notice>(null);
  const [submitting, setSubmitting] = useState(false);
  const navigate = useNavigate();
  const { signIn } = useSession();

  async function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const form = new FormData(event.currentTarget);
    const email = String(form.get("email") ?? "").trim();
    const password = String(form.get("password") ?? "");

    if (!email || !password) {
      setNotice({ tone: "error", text: "Enter your email and password." });
      return;
    }

    setSubmitting(true);
    try {
      await signIn(email, password);
      setNotice({
        tone: "success",
        text: `Welcome back.`,
      });
      navigate("/courses");
    } catch (error) {
      setNotice({
        tone: "error",
        text: error instanceof Error ? error.message : "Sign-in failed.",
      });
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <main className="login-page">
      <section className="login-card" aria-labelledby="page-title">
        <header className="brand">
          <h1 id="page-title">University Course Management System</h1>
          <p>Connecting Minds, Shaping Tomorrow</p>
        </header>

        <div className="illustration-panel" aria-hidden="true">
          <img className="illustration-backdrop" src={`${assetPathPrefix}/804da.svg`} alt="" />
          <img className="illustration" src={`${assetPathPrefix}/23ae0.png`} alt="" />
        </div>

        <div className="form-panel">
          <div className="welcome-copy">
            <h2>WELCOME</h2>
            <p>Enter your User Name and Password to access your account</p>
          </div>

          <form onSubmit={handleSubmit} noValidate>
            <div className="field-group">
              <label htmlFor="email">Email address</label>
              <div className="field-row">
                <img src={`${assetPathPrefix}/eed30.png`} alt="" />
                <input
                  id="email"
                  name="email"
                  type="email"
                  autoComplete="email"
                  placeholder="Enter your university email"
                  aria-describedby="form-notice"
                />
              </div>
            </div>

            <div className="field-group">
              <label htmlFor="password">Password</label>
              <div className="field-row">
                <img src={`${assetPathPrefix}/78c8c.png`} alt="" />
                <input
                  id="password"
                  name="password"
                  type="password"
                  autoComplete="current-password"
                  placeholder="Enter your password"
                  aria-describedby="form-notice"
                />
              </div>
            </div>

            <button className="primary-button" type="submit" disabled={submitting}>
              {submitting ? "Signing in..." : "Sign In"}
            </button>

            <div
              id="form-notice"
              className={`notice${notice ? ` notice-${notice.tone}` : ""}`}
              role="status"
              aria-live="polite"
            >
              {notice?.text ?? ""}
            </div>
          </form>
        </div>
      </section>
    </main>
  );
}
