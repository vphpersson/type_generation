package tag

import "strings"

type Tag struct {
	Name            string
	Skip            bool
	Unique          bool
	Nullable        bool
	OnDelete        string
	OnUpdate        string
	Default         string
	Check           string
	Generated       string
	GeneratedStored string
	Type            string
	OtherOptions    []string
}

func splitTopCommas(s string) []string {
	var out []string
	start := 0
	depth := 0
	inSQ, inDQ := false, false

	for i := 0; i < len(s); i++ {
		c := s[i]

		// handle single-quoted SQL strings with '' escape
		if !inDQ && c == '\'' {
			if inSQ {
				// check doubled quote escape
				if i+1 < len(s) && s[i+1] == '\'' {
					i++ // skip escaped quote
					continue
				}
				inSQ = false
			} else if depth >= 0 { // allow quotes anywhere
				inSQ = true
			}
			continue
		}

		// handle double quotes if you allow them for tag-level quoting
		if !inSQ && c == '"' {
			inDQ = !inDQ
			continue
		}

		if inSQ || inDQ {
			continue
		}

		switch c {
		case '(':
			depth++
		case ')':
			if depth > 0 {
				depth--
			}
		case ',':
			if depth == 0 {
				part := strings.TrimSpace(s[start:i])
				if part != "" {
					out = append(out, part)
				}
				start = i + 1
			}
		}
	}
	last := strings.TrimSpace(s[start:])
	if last != "" {
		out = append(out, last)
	}
	return out
}

func New(tagString string) *Tag {
	trimmedTagString := strings.TrimSpace(tagString)
	if trimmedTagString == "" {
		return nil
	}

	var tag Tag

	elements := splitTopCommas(trimmedTagString)
	if len(elements) == 0 {
		return nil
	}

	if len(elements) == 1 && elements[0] == "-" {
		tag.Skip = true
		return &tag
	}

	tag.Name = elements[0]

	for _, option := range elements[1:] {
		switch lowerOption := strings.ToLower(option); lowerOption {
		case "unique":
			tag.Unique = true
		case "nullable":
			tag.Nullable = true
		default:
			key, value, ok := strings.Cut(option, ":")
			if ok {
				switch strings.ToLower(key) {
				case "default":
					tag.Default = value
					continue
				case "check":
					tag.Check = value
					continue
				case "ondelete":
					tag.OnDelete = value
					continue
				case "onupdate":
					tag.OnUpdate = value
					continue
				case "generated":
					tag.Generated = value
					continue
				case "generatedstored":
					tag.GeneratedStored = value
					continue
				case "type":
					tag.Type = value
					continue
				}
			}
			tag.OtherOptions = append(tag.OtherOptions, option)
		}
	}

	return &tag
}
