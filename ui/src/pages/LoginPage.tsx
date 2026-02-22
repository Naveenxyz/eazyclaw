import { useState, type KeyboardEvent } from "react";
import { useNavigate } from "react-router-dom";
import { useAuth } from "@/hooks/use-auth";

export default function LoginPage() {
  const [password, setPassword] = useState("");
  const { login, loading, error } = useAuth();
  const navigate = useNavigate();

  const handleLogin = async () => {
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
    <div
      className="flex min-h-screen items-center justify-center"
      style={{
        background: `
          radial-gradient(ellipse 80% 60% at 50% 40%, rgba(139, 92, 246, 0.12) 0%, transparent 60%),
          radial-gradient(ellipse 60% 50% at 30% 60%, rgba(34, 211, 238, 0.08) 0%, transparent 50%),
          radial-gradient(ellipse 50% 40% at 70% 30%, rgba(167, 139, 250, 0.06) 0%, transparent 50%),
          #08090d
        `,
      }}
    >
      {/* Animated gradient orb */}
      <div
        className="pointer-events-none fixed inset-0 overflow-hidden"
        aria-hidden="true"
      >
        <div
          className="absolute rounded-full opacity-20 blur-3xl"
          style={{
            width: 500,
            height: 500,
            top: '20%',
            left: '50%',
            transform: 'translateX(-50%)',
            background: 'linear-gradient(135deg, #8b5cf6, #22d3ee)',
            animation: 'pulse-glow 6s ease-in-out infinite',
          }}
        />
      </div>

      <style>{`
        @keyframes pulse-glow {
          0%, 100% { opacity: 0.15; transform: translateX(-50%) scale(1); }
          50% { opacity: 0.25; transform: translateX(-50%) scale(1.05); }
        }
      `}</style>

      <div className="glass-card relative z-10 w-full max-w-sm p-8">
        <h1 className="mb-1 text-center text-2xl font-bold font-mono text-violet-500">
          EazyClaw
        </h1>
        <p className="mb-6 text-center text-sm text-zinc-400">
          Enter your password to continue
        </p>

        <div className="mb-4">
          <label
            htmlFor="password"
            className="mb-1.5 block text-sm font-medium text-zinc-300"
          >
            Password
          </label>
          <input
            id="password"
            type="password"
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            onKeyDown={handleKeyDown}
            placeholder="Enter password"
            className="w-full rounded-md border border-white/10 bg-[#0f1117] px-3 py-2 text-sm text-zinc-100 placeholder-zinc-500 outline-none transition-colors focus:border-violet-500 focus:ring-1 focus:ring-violet-500"
          />
        </div>

        {error && (
          <p className="mb-4 text-center text-sm text-red-400">{error}</p>
        )}

        <button
          onClick={handleLogin}
          disabled={loading}
          className="w-full rounded-md px-4 py-2 text-sm font-medium text-white transition-all disabled:opacity-50"
          style={{
            background: 'linear-gradient(135deg, #8b5cf6, #7c3aed)',
          }}
          onMouseEnter={(e) => {
            (e.target as HTMLButtonElement).style.background = 'linear-gradient(135deg, #a78bfa, #8b5cf6)';
          }}
          onMouseLeave={(e) => {
            (e.target as HTMLButtonElement).style.background = 'linear-gradient(135deg, #8b5cf6, #7c3aed)';
          }}
        >
          {loading ? "Signing in..." : "Sign In"}
        </button>
      </div>
    </div>
  );
}
