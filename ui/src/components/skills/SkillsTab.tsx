import { useState, useEffect } from "react";
import { Puzzle } from "lucide-react";
import { getSkills } from "@/lib/api";
import { SkillCard } from "@/components/skills/SkillCard";
import type { Skill } from "@/types";

export default function SkillsTab() {
  const [skills, setSkills] = useState<Skill[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    getSkills()
      .then((data) => {
        setSkills(data);
        setError(null);
      })
      .catch((err) => {
        setError(err instanceof Error ? err.message : "Failed to load skills");
      })
      .finally(() => {
        setLoading(false);
      });
  }, []);

  if (loading) {
    return (
      <div className="flex h-full items-center justify-center bg-base">
        <div className="flex items-center gap-2">
          <div className="h-3.5 w-3.5 animate-spin rounded-full border-2 border-accent border-t-transparent" />
          <p className="text-xs font-mono text-fg-3">Loading skills...</p>
        </div>
      </div>
    );
  }

  if (error) {
    return (
      <div className="flex h-full items-center justify-center bg-base">
        <p className="text-sm text-error">{error}</p>
      </div>
    );
  }

  if (skills.length === 0) {
    return (
      <div className="flex h-full flex-col items-center justify-center bg-base gap-2">
        <Puzzle size={40} className="text-fg-3 opacity-30" />
        <p className="font-display font-semibold text-fg text-sm mt-2">No skills installed</p>
        <p className="text-fg-3 text-xs font-mono max-w-xs text-center leading-relaxed">
          Drop skill packages into <span className="text-fg-2">/data/skills/</span> to extend the agent with custom tools and instructions.
        </p>
      </div>
    );
  }

  return (
    <div className="p-6 bg-base min-h-full">
      <div className="flex items-center gap-2.5 mb-5">
        <h2 className="section-label">Skills</h2>
        <span className="badge-neutral">{skills.length}</span>
      </div>

      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4 stagger">
        {skills.map((skill) => (
          <SkillCard key={skill.name} skill={skill} />
        ))}
      </div>
    </div>
  );
}
