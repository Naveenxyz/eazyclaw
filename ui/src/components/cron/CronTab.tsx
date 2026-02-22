import { useState, useEffect, useCallback } from "react";
import { Clock, Plus, Pencil, Trash2, X } from "lucide-react";
import { getCronJobs, addCronJob, toggleCronJob, deleteCronJob } from "@/lib/api";
import type { CronJob } from "@/types";

const GO_ZERO_TIME = "0001-01-01T00:00:00Z";

function formatRelativeTime(iso: string): string {
  if (!iso || iso === GO_ZERO_TIME) return "Never";
  const now = Date.now();
  const then = new Date(iso).getTime();
  const diff = now - then;
  const abs = Math.abs(diff);
  const future = diff < 0;

  if (abs < 60_000) return future ? "in <1m" : "<1m ago";
  if (abs < 3_600_000) {
    const m = Math.floor(abs / 60_000);
    return future ? `in ${m}m` : `${m}m ago`;
  }
  if (abs < 86_400_000) {
    const h = Math.floor(abs / 3_600_000);
    return future ? `in ${h}h` : `${h}h ago`;
  }
  const d = Math.floor(abs / 86_400_000);
  return future ? `in ${d}d` : `${d}d ago`;
}

export default function CronTab() {
  const [jobs, setJobs] = useState<CronJob[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [showModal, setShowModal] = useState(false);
  const [newSchedule, setNewSchedule] = useState("");
  const [newTask, setNewTask] = useState("");
  const [creating, setCreating] = useState(false);

  const fetchJobs = useCallback(() => {
    getCronJobs()
      .then((data) => {
        setJobs(Array.isArray(data) ? data : []);
        setError(null);
      })
      .catch((err) => {
        setError(err instanceof Error ? err.message : "Failed to load cron jobs");
      })
      .finally(() => {
        setLoading(false);
      });
  }, []);

  useEffect(() => {
    fetchJobs();
    const interval = setInterval(fetchJobs, 5000);
    return () => clearInterval(interval);
  }, [fetchJobs]);

  const handleToggle = async (id: string) => {
    try {
      const result = await toggleCronJob(id);
      setJobs((prev) =>
        prev.map((j) => (j.id === id ? { ...j, enabled: result.enabled } : j))
      );
    } catch {
      // toggle failed silently, next refresh will correct state
    }
  };

  const handleDelete = async (id: string) => {
    if (!window.confirm("Delete this cron job? This action cannot be undone.")) return;
    try {
      await deleteCronJob(id);
      setJobs((prev) => prev.filter((j) => j.id !== id));
    } catch {
      // delete failed, next refresh will correct state
    }
  };

  const handleCreate = async () => {
    if (!newSchedule.trim() || !newTask.trim()) return;
    setCreating(true);
    try {
      await addCronJob(newSchedule.trim(), newTask.trim());
      setNewSchedule("");
      setNewTask("");
      setShowModal(false);
      fetchJobs();
    } catch {
      // create failed
    } finally {
      setCreating(false);
    }
  };

  // --- Loading state ---
  if (loading) {
    return (
      <div className="flex h-full items-center justify-center bg-base">
        <p className="text-xs font-mono text-fg-3">Loading cron jobs...</p>
      </div>
    );
  }

  // --- Error state ---
  if (error) {
    return (
      <div className="flex h-full items-center justify-center bg-base">
        <p className="text-sm text-error">{error}</p>
      </div>
    );
  }

  return (
    <div className="p-6 bg-base min-h-full">
      {/* Header */}
      <div className="flex items-center justify-between mb-6">
        <div className="flex items-center gap-3">
          <h2 className="section-label">Cron Jobs</h2>
          <span className="badge-neutral">{jobs.length}</span>
        </div>
        <button
          onClick={() => setShowModal(true)}
          className="btn btn-accent flex items-center gap-1.5"
        >
          <Plus size={14} />
          New Job
        </button>
      </div>

      {/* Empty state */}
      {jobs.length === 0 && (
        <div className="flex flex-col items-center justify-center py-24">
          <Clock size={40} className="text-fg-3 opacity-30" />
          <p className="section-label mt-4">No cron jobs scheduled</p>
          <p className="text-fg-3 text-xs font-mono mt-2">
            Create your first scheduled task
          </p>
        </div>
      )}

      {/* Job list */}
      {jobs.length > 0 && (
        <div className="flex flex-col gap-3 stagger">
          {jobs.map((job) => (
            <div key={job.id} className="card card-accent p-4">
              {/* Row 1: Schedule + Status pill */}
              <div className="flex items-center justify-between mb-2">
                <span className="font-mono text-accent text-sm font-medium">
                  {job.schedule}
                </span>
                <span
                  className={`cron-status ${job.enabled ? "active" : "disabled"}`}
                >
                  {job.enabled ? "Active" : "Disabled"}
                </span>
              </div>

              {/* Row 2: Task description */}
              <p className="text-sm text-fg line-clamp-2 mb-3">{job.task}</p>

              {/* Row 3: Metadata */}
              <div className="flex items-center gap-4 text-xs text-fg-3 font-mono mb-3">
                <span>Last: {formatRelativeTime(job.last_run)}</span>
                <span>Next: {formatRelativeTime(job.next_run)}</span>
              </div>

              {/* Row 4: Actions */}
              <div className="flex items-center gap-2">
                <button
                  onClick={() => handleToggle(job.id)}
                  className={`toggle ${job.enabled ? "active" : ""}`}
                  aria-label={job.enabled ? "Disable job" : "Enable job"}
                />
                <button
                  onClick={() => {
                    /* edit not yet wired */
                  }}
                  className="btn p-1.5 text-fg-3 hover:text-fg transition-colors"
                  aria-label="Edit job"
                >
                  <Pencil size={14} />
                </button>
                <button
                  onClick={() => handleDelete(job.id)}
                  className="btn btn-danger p-1.5"
                  aria-label="Delete job"
                >
                  <Trash2 size={14} />
                </button>
              </div>
            </div>
          ))}
        </div>
      )}

      {/* New Job Modal */}
      {showModal && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-base/80 backdrop-blur-sm">
          <div className="card max-w-md w-full mx-4 p-6">
            {/* Modal header */}
            <div className="flex items-center justify-between mb-5">
              <h3 className="font-display font-semibold text-fg">New Cron Job</h3>
              <button
                onClick={() => setShowModal(false)}
                className="text-fg-3 hover:text-fg transition-colors"
              >
                <X size={18} />
              </button>
            </div>

            {/* Schedule input */}
            <div className="mb-4">
              <label className="block text-xs text-fg-2 font-medium mb-1.5">
                Schedule
              </label>
              <input
                type="text"
                value={newSchedule}
                onChange={(e) => setNewSchedule(e.target.value)}
                placeholder="0 9 * * *"
                className="input-focus w-full rounded-md bg-raised border border-edge px-3 py-2 text-sm font-mono text-fg placeholder:text-fg-3"
              />
              <p className="text-xs text-fg-3 mt-1.5">
                Use standard 5-field cron format: MIN HOUR DOM MON DOW
              </p>
            </div>

            {/* Task textarea */}
            <div className="mb-5">
              <label className="block text-xs text-fg-2 font-medium mb-1.5">
                Task
              </label>
              <textarea
                value={newTask}
                onChange={(e) => setNewTask(e.target.value)}
                placeholder="Describe the task..."
                rows={3}
                className="input-focus w-full rounded-md bg-raised border border-edge px-3 py-2 text-sm text-fg placeholder:text-fg-3 resize-none"
              />
            </div>

            {/* Actions */}
            <div className="flex items-center justify-end gap-3">
              <button
                onClick={() => setShowModal(false)}
                className="btn px-4 py-2 text-sm text-fg-2"
              >
                Cancel
              </button>
              <button
                onClick={handleCreate}
                disabled={creating || !newSchedule.trim() || !newTask.trim()}
                className="btn btn-accent px-4 py-2 text-sm disabled:opacity-40"
              >
                {creating ? "Creating..." : "Create"}
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
