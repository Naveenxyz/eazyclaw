import type { Skill } from "@/types";

interface SkillCardProps {
  skill: Skill;
}

export function SkillCard({ skill }: SkillCardProps) {
  return (
    <div className="glass-card rounded-lg border border-violet-500/10 bg-[#0f1117] p-5 transition-all hover:border-violet-500/30 hover:shadow-[0_0_15px_-3px] hover:shadow-violet-500/10">
      <h3 className="text-lg font-semibold text-slate-200">{skill.name}</h3>
      {skill.description && (
        <p className="mt-1 text-sm text-slate-400">{skill.description}</p>
      )}

      {skill.tools && skill.tools.length > 0 && (
        <div className="mt-4">
          <h4 className="mb-2 text-xs font-mono font-semibold uppercase tracking-wider text-slate-500">
            Tools
          </h4>
          <div className="flex flex-wrap gap-2">
            {skill.tools.map((tool) => (
              <span
                key={tool.name}
                className="inline-flex items-center px-2 py-0.5 rounded-full text-xs font-mono bg-cyan-500/10 text-cyan-400 border border-cyan-500/20"
              >
                {tool.name}
              </span>
            ))}
          </div>
        </div>
      )}

      {skill.dependencies && skill.dependencies.length > 0 && (
        <div className="mt-4">
          <h4 className="mb-2 text-xs font-mono font-semibold uppercase tracking-wider text-slate-500">
            Dependencies
          </h4>
          <div className="flex flex-wrap gap-2">
            {skill.dependencies.map((dep) => (
              <span
                key={`${dep.manager}:${dep.package}`}
                className="inline-flex items-center px-2 py-0.5 rounded-full text-xs font-mono bg-violet-500/10 text-violet-400 border border-violet-500/20"
              >
                {dep.manager}: {dep.package}
              </span>
            ))}
          </div>
        </div>
      )}
    </div>
  );
}
