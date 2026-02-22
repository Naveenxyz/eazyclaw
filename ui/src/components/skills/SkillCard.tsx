import type { Skill } from "@/types";

interface SkillCardProps {
  skill: Skill;
}

export function SkillCard({ skill }: SkillCardProps) {
  return (
    <div className="card card-accent p-4">
      <h3 className="font-display font-semibold text-fg text-sm">{skill.name}</h3>

      {skill.description && (
        <p className="mt-1 text-xs text-fg-2 italic line-clamp-2">{skill.description}</p>
      )}

      {skill.tools && skill.tools.length > 0 && (
        <div className="mt-3">
          <span className="section-label text-[10px] mb-1.5 block">Tools</span>
          <div className="flex flex-wrap gap-1.5">
            {skill.tools.map((tool) => (
              <span key={tool.name} className="badge-accent">
                {tool.name}
              </span>
            ))}
          </div>
        </div>
      )}

      {skill.dependencies && skill.dependencies.length > 0 && (
        <div className="mt-3">
          <span className="section-label text-[10px] mb-1.5 block">Deps</span>
          <div className="flex flex-wrap gap-1.5">
            {skill.dependencies.map((dep) => (
              <span key={`${dep.manager}:${dep.package}`} className="badge-info">
                {dep.manager}:{dep.package}
              </span>
            ))}
          </div>
        </div>
      )}
    </div>
  );
}
