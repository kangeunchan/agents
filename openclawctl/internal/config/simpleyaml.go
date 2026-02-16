package config

import (
	"bytes"
	"fmt"
	"io"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

func ParseYAML(data []byte) (map[string]any, error) {
	decoder := yaml.NewDecoder(bytes.NewReader(data))

	var doc yaml.Node
	if err := decoder.Decode(&doc); err != nil {
		if err == io.EOF {
			return map[string]any{}, nil
		}
		return nil, fmt.Errorf("decode yaml: %w", err)
	}
	if doc.Kind == 0 || len(doc.Content) == 0 {
		return map[string]any{}, nil
	}

	if err := detectDuplicateKeys(&doc, nil); err != nil {
		return nil, err
	}

	var out map[string]any
	if err := doc.Decode(&out); err != nil {
		return nil, fmt.Errorf("decode yaml document: %w", err)
	}
	if out == nil {
		out = map[string]any{}
	}

	var extra yaml.Node
	if err := decoder.Decode(&extra); err != nil && err != io.EOF {
		return nil, fmt.Errorf("decode trailing yaml document: %w", err)
	}
	if extra.Kind != 0 {
		return nil, fmt.Errorf("multiple yaml documents are not supported")
	}

	return out, nil
}

func detectDuplicateKeys(node *yaml.Node, path []string) error {
	if node == nil {
		return nil
	}

	switch node.Kind {
	case yaml.DocumentNode:
		for _, child := range node.Content {
			if err := detectDuplicateKeys(child, path); err != nil {
				return err
			}
		}
	case yaml.MappingNode:
		seen := map[string]int{}
		for i := 0; i+1 < len(node.Content); i += 2 {
			keyNode := node.Content[i]
			valueNode := node.Content[i+1]
			key := strings.TrimSpace(keyNode.Value)
			if firstLine, exists := seen[key]; exists {
				return fmt.Errorf(
					"duplicate key %q at line %d (first defined at line %d)%s",
					key,
					keyNode.Line,
					firstLine,
					formatYAMLPath(path),
				)
			}
			seen[key] = keyNode.Line

			nextPath := append(append([]string{}, path...), key)
			if err := detectDuplicateKeys(valueNode, nextPath); err != nil {
				return err
			}
		}
	case yaml.SequenceNode:
		for i, item := range node.Content {
			nextPath := append(append([]string{}, path...), "["+strconv.Itoa(i)+"]")
			if err := detectDuplicateKeys(item, nextPath); err != nil {
				return err
			}
		}
	}
	return nil
}

func formatYAMLPath(path []string) string {
	if len(path) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString(" at ")
	for i, part := range path {
		if strings.HasPrefix(part, "[") {
			b.WriteString(part)
			continue
		}
		if i > 0 {
			b.WriteString(".")
		}
		b.WriteString(part)
	}
	return b.String()
}
