package tag

import (
	"fmt"
	"strconv"
	"strings"

	motmedelErrors "github.com/Motmedel/utils_go/pkg/errors"
)

type Tag struct {
	Name         string
	Skip         bool
	Optional     bool
	MinLength    int
	MaxLength    int
	Minimum      float64
	Maximum      float64
	Format       string
	OtherOptions []string
}

func New(tagString string) (*Tag, error) {
	trimmedTagString := strings.TrimSpace(tagString)
	if trimmedTagString == "" {
		return nil, nil
	}

	var tag Tag

	elements := strings.Split(strings.TrimSpace(trimmedTagString), ",")
	if len(elements) == 0 {
		return nil, nil
	}

	if len(elements) == 1 && elements[0] == "-" {
		tag.Skip = true
		return &tag, nil
	}

	tag.Name = elements[0]

	for _, option := range elements[1:] {
		option = strings.ToLower(strings.TrimSpace(option))
		switch option {
		case "optional":
			tag.Optional = true
		default:
			key, value, ok := strings.Cut(option, ":")
			if ok {
				var err error
				switch strings.ToLower(key) {
				case "format":
					tag.Format = value
					continue
				case "minlength":
					tag.MinLength, err = strconv.Atoi(value)
					if err != nil {
						return nil, motmedelErrors.NewWithTrace(fmt.Errorf("strconv atoi (minlength): %w", err))
					}
					continue
				case "maxlength":
					tag.MaxLength, err = strconv.Atoi(value)
					if err != nil {
						return nil, motmedelErrors.NewWithTrace(fmt.Errorf("strconv atoi (maxlength): %w", err))
					}
					continue
				case "minimum":
					tag.Minimum, err = strconv.ParseFloat(value, 64)
					if err != nil {
						return nil, motmedelErrors.NewWithTrace(fmt.Errorf("strconv parse float (minimum): %w", err))
					}
					continue
				case "maximum":
					tag.Maximum, err = strconv.ParseFloat(value, 64)
					if err != nil {
						return nil, motmedelErrors.NewWithTrace(fmt.Errorf("strconv parse float (maximum): %w", err))
					}
					continue
				}
			}
			tag.OtherOptions = append(tag.OtherOptions, option)
		}
	}

	return &tag, nil
}
