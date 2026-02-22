import type { Skill } from "@/types";

interface SkillCardProps {
  skill: Skill;
}

export function SkillCard({ skill }: SkillCardProps) {
  return (
    <div className="rounded-lg border border-zinc-800 bg-zinc-900 p-5">
      <h3 className="text-lg font-semibold text-violet-400">{skill.name}</h3>
      {skill.description && (
        <p className="mt-1 text-sm text-zinc-400">{skill.description}</p>
      )}

      {skill.tools && skill.tools.length > 0 && (
        <div className="mt-4">
          <h4 className="mb-2 text-xs font-semibold uppercase tracking-wider text-zinc-500">
            Tools
          </h4>
          <ul className="flex flex-col gap-2">
            {skill.tools.map((tool) => (
              <li key={tool.name}>
                <span className="font-bold text-violet-400">{tool.name}</span>
                {tool.description && (
                  <span className="ml-2 text-sm text-zinc-400">
                    {tool.description}
                  </span>
                )}
              </li>
            ))}
          </ul>
        </div>
      )}

      {skill.dependencies && skill.dependencies.length > 0 && (
        <div className="mt-4">
          <h4 className="mb-2 text-xs font-semibold uppercase tracking-wider text-zinc-500">
            Dependencies
          </h4>
          <div className="flex flex-wrap gap-2">
            {skill.dependencies.map((dep) => (
              <span
                key={`${dep.manager}:${dep.package}`}
                className="rounded-full bg-zinc-800 px-3 py-1 text-xs text-zinc-300"
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
