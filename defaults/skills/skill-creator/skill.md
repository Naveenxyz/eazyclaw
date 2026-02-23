# Skill: skill-creator

## Description
Create, update, and manage EazyClaw skills. Use when designing new skills, packaging agent capabilities, or extending the agent with new workflows. A skill is a folder with a skill.md file placed in /data/eazyclaw/skills/.

## Instructions
You can create new skills to extend your own capabilities. Skills are modular packages that teach you new workflows, tool patterns, and domain expertise.

### Skill Format

Every skill lives at `/data/eazyclaw/skills/<skill-name>/skill.md`. The file uses markdown with these sections (each section starts with a double-hash heading):

**Section 1 — Title line**: `# Skill: <name>` (top-level heading with skill name)

**Section 2 — Description**: Short summary shown in the UI. Include trigger phrases ("Use when..."). This text helps the system decide when to activate the skill.

**Section 3 — Instructions**: The main content injected into the system prompt. Write detailed workflows, commands, examples, and decision trees here. This is where you teach yourself how to use the skill.

**Section 4 — Tools**: Shell command templates. Each tool has three fields indented under a list item:
  - `- name:` the tool identifier
  - `  description:` what it does
  - `  command:` the shell command template with `{{placeholder}}` for variables

**Section 5 — Dependencies**: Package requirements as list items:
  - `- pip: <package>` for Python packages
  - `- npm: <package>` for Node packages

### Example skill.md (structure only)

The file starts with `# Skill: my-tool`, then has Description, Instructions, Tools, and Dependencies sections separated by double-hash headings. Write each section heading as `## SectionName` on its own line.

### Creating a New Skill

1. **Understand the need**: What capability is missing? What would you do repeatedly?
2. **Name it**: Use lowercase-hyphenated names under 64 chars. Verb-led is best (`fetch-rss`, `analyze-logs`, `deploy-app`).
3. **Create the directory**:
   ```
   shell: mkdir -p /data/eazyclaw/skills/<skill-name>
   ```
4. **Write skill.md** using the `write_file` tool to `/data/eazyclaw/skills/<skill-name>/skill.md`
5. **Restart required**: The skill loads at container startup. Inform the user a restart is needed.

### Design Principles

- **Be concise**: The context window is shared. Only include what you don't already know.
- **Be specific**: Include exact commands, not vague descriptions.
- **Include examples**: Show input/output for common use cases.
- **Set freedom levels**: Fragile operations need exact commands. Flexible tasks need heuristics.
- **No fluff**: Skip README files, changelogs, installation guides. Just skill.md.

### Updating an Existing Skill

Use `edit_file` to modify `/data/eazyclaw/skills/<skill-name>/skill.md`. Changes take effect on container restart.

### Listing Skills

Use `list_dir` on `/data/eazyclaw/skills/` to see installed skills. Each subdirectory with a `skill.md` is a skill.

### Skill Ideas

When the user asks you to learn something or automate a workflow, consider creating a skill for it. Good candidates:
- Repetitive multi-step processes
- Domain-specific knowledge (APIs, formats, protocols)
- Tool combinations that work well together
- Project-specific conventions and patterns

## Tools
- name: init_skill
  description: Create a new skill directory with template skill.md
  command: mkdir -p /data/eazyclaw/skills/{{skill_name}}
- name: list_skills
  description: List all installed skills
  command: ls -1 /data/eazyclaw/skills/
- name: read_skill
  description: Read a skill definition
  command: cat /data/eazyclaw/skills/{{skill_name}}/skill.md
