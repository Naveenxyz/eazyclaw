package skill

import (
	"strings"
)

// ParseSkillMD parses a skill.md file into a Skill struct.
//
// Expected format:
//
//	# Skill: <name>
//
//	## Description
//	<description text>
//
//	## Instructions
//	<instructions text>
//
//	## Tools
//	- name: <tool_name>
//	  description: <desc>
//	  command: <cmd template>
//
//	## Dependencies
//	- npm: <package>
//	- pip: <package>
func ParseSkillMD(content string) (*Skill, error) {
	skill := &Skill{}
	lines := strings.Split(content, "\n")

	// Extract skill name from the top-level heading.
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "# Skill:") {
			skill.Name = strings.TrimSpace(strings.TrimPrefix(trimmed, "# Skill:"))
			break
		}
	}

	// Split content into sections by ## headings.
	sections := parseSections(lines)

	if desc, ok := sections["Description"]; ok {
		skill.Description = strings.TrimSpace(desc)
	}

	if instr, ok := sections["Instructions"]; ok {
		skill.Instructions = strings.TrimSpace(instr)
	}

	if toolsSection, ok := sections["Tools"]; ok {
		skill.Tools = parseTools(toolsSection)
	}

	if depsSection, ok := sections["Dependencies"]; ok {
		skill.Dependencies = parseDependencies(depsSection)
	}

	return skill, nil
}

// parseSections splits lines into sections keyed by ## heading names.
// It skips ## headings inside fenced code blocks (``` delimiters).
func parseSections(lines []string) map[string]string {
	sections := make(map[string]string)
	var currentSection string
	var currentContent []string
	inCodeBlock := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Track fenced code block boundaries.
		if strings.HasPrefix(trimmed, "```") {
			inCodeBlock = !inCodeBlock
		}

		if !inCodeBlock && strings.HasPrefix(trimmed, "## ") {
			// Save previous section.
			if currentSection != "" {
				sections[currentSection] = strings.Join(currentContent, "\n")
			}
			currentSection = strings.TrimSpace(strings.TrimPrefix(trimmed, "## "))
			currentContent = nil
			continue
		}
		if currentSection != "" {
			currentContent = append(currentContent, line)
		}
	}

	// Save last section.
	if currentSection != "" {
		sections[currentSection] = strings.Join(currentContent, "\n")
	}

	return sections
}

// parseTools extracts tool definitions from the Tools section content.
// Tools are defined as YAML-like list items:
//
//	- name: <tool_name>
//	  description: <desc>
//	  command: <cmd template>
func parseTools(content string) []SkillTool {
	var tools []SkillTool
	lines := strings.Split(content, "\n")

	var current *SkillTool
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}

		if strings.HasPrefix(trimmed, "- name:") {
			// Save previous tool.
			if current != nil {
				tools = append(tools, *current)
			}
			current = &SkillTool{
				Name: strings.TrimSpace(strings.TrimPrefix(trimmed, "- name:")),
			}
			continue
		}

		if current == nil {
			continue
		}

		if strings.HasPrefix(trimmed, "description:") {
			current.Description = strings.TrimSpace(strings.TrimPrefix(trimmed, "description:"))
		} else if strings.HasPrefix(trimmed, "command:") {
			current.Command = strings.TrimSpace(strings.TrimPrefix(trimmed, "command:"))
		}
	}

	// Save last tool.
	if current != nil {
		tools = append(tools, *current)
	}

	return tools
}

// parseDependencies extracts dependency definitions from the Dependencies section content.
// Dependencies are defined as list items:
//
//	- npm: <package>
//	- pip: <package>
func parseDependencies(content string) []Dependency {
	var deps []Dependency
	lines := strings.Split(content, "\n")

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "- ") {
			continue
		}
		trimmed = strings.TrimPrefix(trimmed, "- ")

		if strings.HasPrefix(trimmed, "npm:") {
			deps = append(deps, Dependency{
				Manager: "npm",
				Package: strings.TrimSpace(strings.TrimPrefix(trimmed, "npm:")),
			})
		} else if strings.HasPrefix(trimmed, "pip:") {
			deps = append(deps, Dependency{
				Manager: "pip",
				Package: strings.TrimSpace(strings.TrimPrefix(trimmed, "pip:")),
			})
		}
	}

	return deps
}
