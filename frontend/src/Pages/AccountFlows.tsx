import { FormEvent, useState } from "react";
import { Link } from "react-router-dom";
import {
  activateAccount,
  registerUser,
  requestPasswordReset,
  resetPassword,
} from "../services/auth";
import "./AccountFlows.css";

type AccountMode = "register" | "forgot" | "reset" | "activate";

function AccountPage({
  mode,
  title,
  description,
  submitLabel,
  onSubmit,
}: {
  mode: AccountMode;
  title: string;
  description: string;
  submitLabel: string;
  onSubmit: (data: FormData) => Promise<string>;
}) {
  const [notice, setNotice] = useState("");
  const [error, setError] = useState("");
  const [submitting, setSubmitting] = useState(false);

  async function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setNotice("");
    setError("");
    setSubmitting(true);
    try {
      setNotice(await onSubmit(new FormData(event.currentTarget)));
    } catch (requestError) {
      setError(requestError instanceof Error ? requestError.message : "The request failed.");
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <main className="account-page">
      <section className="account-card" aria-labelledby="account-title">
        <h1 id="account-title">{title}</h1>
        <p>{description}</p>
        <form onSubmit={handleSubmit}>
          {mode === "register" && (
            <>
              <label>
                First name
                <input name="first_name" autoComplete="given-name" required maxLength={100} />
              </label>
              <label>
                Last name
                <input name="last_name" autoComplete="family-name" required maxLength={100} />
              </label>
            </>
          )}
          {(mode === "register" || mode === "forgot") && (
            <label>
              University email
              <input
                name="email"
                type="email"
                autoComplete="email"
                required
                maxLength={254}
              />
            </label>
          )}
          {(mode === "reset" || mode === "activate") && (
            <>
              <label>
                {mode === "activate" ? "Activation token" : "Password reset token"}
                <input name="token" required defaultValue={tokenFromLocation()} />
              </label>
              <label>
                {mode === "activate" ? "Choose a password" : "New password"}
                <input
                  name="password"
                  type="password"
                  autoComplete="new-password"
                  required
                  minLength={12}
                  maxLength={128}
                />
              </label>
            </>
          )}
          {error && <p className="account-error" role="alert">{error}</p>}
          {notice && <p className="account-notice" role="status">{notice}</p>}
          <button type="submit" disabled={submitting}>
            {submitting ? "Please wait..." : submitLabel}
          </button>
        </form>
        <Link to="/login">Back to sign in</Link>
      </section>
    </main>
  );
}

function tokenFromLocation() {
  return new URLSearchParams(window.location.search).get("token") ?? "";
}

export function RegisterPage() {
  return (
    <AccountPage
      mode="register"
      title="Create an account"
      description="Submit your details for university account approval."
      submitLabel="Submit registration"
      onSubmit={async (data) => {
        await registerUser({
          email: String(data.get("email") ?? "").trim(),
          first_name: String(data.get("first_name") ?? "").trim(),
          last_name: String(data.get("last_name") ?? "").trim(),
        });
        return "Registration submitted. Your account is pending administrative approval.";
      }}
    />
  );
}

export function ForgotPasswordPage() {
  return (
    <AccountPage
      mode="forgot"
      title="Reset your password"
      description="Enter your university email. If the account is eligible, recovery instructions will be sent."
      submitLabel="Send recovery instructions"
      onSubmit={async (data) => {
        await requestPasswordReset(String(data.get("email") ?? "").trim());
        return "If the account is eligible, recovery instructions will be sent.";
      }}
    />
  );
}

function PasswordTokenPage({ mode }: { mode: "reset" | "activate" }) {
  return (
    <AccountPage
      mode={mode}
      title={mode === "activate" ? "Activate your account" : "Choose a new password"}
      description="Use the secure token from your university email."
      submitLabel={mode === "activate" ? "Activate account" : "Update password"}
      onSubmit={async (data) => {
        const token = String(data.get("token") ?? "").trim();
        const password = String(data.get("password") ?? "");
        if (mode === "activate") {
          await activateAccount(token, password);
        } else {
          await resetPassword(token, password);
        }
        return mode === "activate"
          ? "Your account has been activated. You can now sign in."
          : "Your password has been updated. You can now sign in.";
      }}
    />
  );
}

export function ResetPasswordPage() {
  return <PasswordTokenPage mode="reset" />;
}

export function ActivateAccountPage() {
  return <PasswordTokenPage mode="activate" />;
}
