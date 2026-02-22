import { useState, useEffect } from "react";
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
      <div className="flex h-full items-center justify-center bg-[#08090d]">
        <p className="text-sm font-mono text-slate-500">Loading skills...</p>
      </div>
    );
  }

  if (error) {
    return (
      <div className="flex h-full items-center justify-center bg-[#08090d]">
        <p className="text-sm text-red-400">{error}</p>
      </div>
    );
  }

  if (skills.length === 0) {
    return (
      <div className="flex h-full items-center justify-center bg-[#08090d]">
        <p className="text-sm font-mono text-slate-500">No skills loaded</p>
      </div>
    );
  }

  return (
    <div className="p-6 bg-[#08090d]">
      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4 fade-in-stagger">
        {skills.map((skill) => (
          <SkillCard key={skill.name} skill={skill} />
        ))}
      </div>
    </div>
  );
}
