import { useState, type KeyboardEvent } from "react";
import { useNavigate } from "react-router-dom";
import { useAuth } from "@/hooks/use-auth";

export default function LoginPage() {
  const [password, setPassword] = useState("");
  const { login, loading, error } = useAuth();
  const navigate = useNavigate();

  const handleLogin = async () => {
    if (!password.trim() || loading) return;
    const success = await login(password);
    if (success) {
      navigate("/");
    }
  };

  const handleKeyDown = (e: KeyboardEvent<HTMLInputElement>) => {
    if (e.key === "Enter") {
      handleLogin();
    }
  };

  return (
    <div className="grid-texture flex min-h-screen items-center justify-center bg-base relative">
      {/* Ambient glow orb */}
      <div
        className="pointer-events-none fixed"
        aria-hidden="true"
        style={{
          width: 520,
          height: 520,
          top: "50%",
          left: "50%",
          transform: "translate(-50%, -50%)",
          background:
            "radial-gradient(circle, rgba(0, 229, 153, 0.03) 0%, transparent 60%)",
          animation: "drift 20s ease-in-out infinite",
        }}
      />

      {/* Login card */}
      <div className="card card-accent relative z-10 w-full max-w-sm rounded-lg border border-edge bg-surface p-8">
        {/* Title */}
        <h1 className="text-center font-display text-[28px] font-extrabold text-accent tracking-[0.06em]">
          EAZYCLAW
        </h1>

        {/* Accent separator */}
        <div
          className="mx-auto mt-3 mb-8 w-12"
          style={{
            height: 1,
            background: "rgba(0, 229, 153, 0.2)",
          }}
        />

        {/* Subtitle */}
        <p className="mb-8 text-center font-mono text-[11px] uppercase tracking-widest text-fg-3">
          Control Interface
        </p>

        {/* Password field */}
        <div className="mb-4">
          <label
            htmlFor="login-password"
            className="mb-1.5 block text-xs font-medium text-fg-2"
          >
            Password
          </label>
          <input
            id="login-password"
            type="password"
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            onKeyDown={handleKeyDown}
            placeholder="Enter access key"
            autoFocus
            className="input-focus w-full rounded-md border border-edge bg-raised px-3 py-2.5 font-mono text-sm text-fg placeholder:text-fg-3 outline-none"
          />
        </div>

        {/* Error message */}
        {error && (
          <p className="mt-2 mb-3 text-center text-xs text-error">{error}</p>
        )}

        {/* Login button */}
        <button
          onClick={handleLogin}
          disabled={loading || !password.trim()}
          className="btn btn-accent mt-4 w-full"
        >
          {loading ? (
            <span className="flex items-center justify-center gap-2">
              <svg
                className="h-4 w-4 animate-spin"
                viewBox="0 0 24 24"
                fill="none"
              >
                <circle
                  cx="12"
                  cy="12"
                  r="10"
                  stroke="currentColor"
                  strokeWidth="3"
                  strokeLinecap="round"
                  className="opacity-25"
                />
                <path
                  d="M4 12a8 8 0 018-8"
                  stroke="currentColor"
                  strokeWidth="3"
                  strokeLinecap="round"
                />
              </svg>
              Authenticating...
            </span>
          ) : (
            "Login"
          )}
        </button>
      </div>
    </div>
  );
}
