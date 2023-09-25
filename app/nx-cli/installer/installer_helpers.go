package installer

import "strings"

func GetLinuxId(s string) string {
	lines := strings.Split(s, "\n")

	for _, line := range lines {
		parts := strings.Split(line, "=")
		if parts[0] == "ID" {
			return parts[1]
		}
	}

	return ""
}

func GetLinuxIdLike(s string) string {
	lines := strings.Split(s, "\n")

	for _, line := range lines {
		parts := strings.Split(line, "=")
		if parts[0] == "ID_LIKE" {
			return parts[1]
		}
	}

	return ""
}
