package skill

import (
	"fmt"
	"os"
	"path/filepath"
)

// Skill represents a loaded skill with its metadata and tools.
type Skill struct {
	Name         string
	Path         string
	Description  string
	Instructions string
	Tools        []SkillTool
	Dependencies []Dependency
}

// SkillTool represents a tool defined within a skill.
type SkillTool struct {
	Name        string
	Description string
	Command     string
}

// Dependency represents an external package dependency for a skill.
type Dependency struct {
	Manager string // "npm" | "pip"
	Package string
}

// LoadSkills scans a directory for subdirectories containing skill.md files
// and loads each skill.
func LoadSkills(dir string) ([]Skill, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil // No skills directory is not an error.
		}
		return nil, fmt.Errorf("failed to read skills directory: %w", err)
	}

	var skills []Skill
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		skillFile := filepath.Join(dir, entry.Name(), "skill.md")
		data, err := os.ReadFile(skillFile)
		if err != nil {
			continue // Skip directories without skill.md.
		}

		skill, err := ParseSkillMD(string(data))
		if err != nil {
			return nil, fmt.Errorf("failed to parse skill %s: %w", entry.Name(), err)
		}

		// Use directory name as fallback name if not set in the file.
		if skill.Name == "" {
			skill.Name = entry.Name()
		}
		skill.Path = skillFile

		skills = append(skills, *skill)
	}

	return skills, nil
}
