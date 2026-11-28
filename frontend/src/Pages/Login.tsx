import { FormEvent, useState } from "react";
import { useNavigate } from "react-router-dom";
import { login } from "../services/auth";
import UniversityIllustration from "./UniversityIllustration";

const assetPathPrefix = "/assets";

type Notice = {
  tone: "success" | "info" | "error";
  text: string;
} | null;

export default function Login() {
  const [notice, setNotice] = useState<Notice>(null);
  const [submitting, setSubmitting] = useState(false);
  const navigate = useNavigate();

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
      const response = await login(email, password);
      setNotice({
        tone: "success",
        text: `Welcome back, ${response.user.first_name || email}.`,
      });
      const roles = response.user.roles ?? [];
      navigate(
        roles.includes("ADMIN")
          ? "/admin"
          : roles.includes("LECTURER")
            ? "/lecturer/dashboard"
            : "/courses",
      );
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
          <UniversityIllustration />
        </div>

        <div className="form-panel">
          <div className="welcome-copy">
            <h2>WELCOME</h2>
            <p>Enter your university email and password to access your account</p>
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

            <div className="form-options">
              <button
                className="text-button forgot"
                type="button"
                onClick={() => navigate("/forgot-password")}
              >
                Forgot Password
              </button>
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

          <p className="signup-copy">
            <span>Don’t have an account?</span>{" "}
            <button
              className="text-button"
              type="button"
              onClick={() => navigate("/register")}
            >
              Sign up
            </button>
          </p>
        </div>
      </section>
    </main>
  );
}