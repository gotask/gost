// ini.go
package stconfig

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
)

func LoadINI(path string) (*Config, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	cfg := &Config{}
	e := cfg.readIniFile(bufio.NewReader(f))
	if e != nil {
		return nil, e
	}
	cfg.path = path
	return cfg, nil
}

func (config *Config) Save() error {
	f, err := os.Create(config.path)
	if err != nil {
		return err
	}

	for _, key := range config.keys {
		if val, ok := config.values[key]; ok {
			if com, ok := config.commentValues[key]; ok && com != "" {
				f.WriteString("#")
				f.WriteString(com)
				f.WriteString("\n")
			}
			f.WriteString(key)
			f.WriteString(" = ")

			nv := strings.TrimSpace(val)
			isSpace := len(nv) != len(val)
			if isSpace {
				f.WriteString("\"")
			}
			f.WriteString(val)
			if isSpace {
				f.WriteString("\"")
			}

			f.WriteString("\n")
		}
	}

	for _, seckey := range config.sectionkeys {
		if sec, ok := config.sections[seckey]; ok {
			f.WriteString("\n")
			if sec.commentSection != "" {
				f.WriteString("#")
				f.WriteString(sec.commentSection)
				f.WriteString("\n")
			}
			f.WriteString("[")
			f.WriteString(sec.name)
			f.WriteString("]\n")
			for _, key := range sec.keys {
				if val, ok := sec.values[key]; ok {
					if com, ok := sec.commentValues[key]; ok && com != "" {
						f.WriteString("#")
						f.WriteString(com)
						f.WriteString("\n")
					}
					f.WriteString(key)
					f.WriteString(" = ")

					nv := strings.TrimSpace(val)
					isSpace := len(nv) != len(val)
					if isSpace {
						f.WriteString("\"")
					}
					f.WriteString(val)
					if isSpace {
						f.WriteString("\"")
					}

					f.WriteString("\n")
				}
			}
		}
	}

	f.Close()
	return nil
}

func (config *configSection) set(key, val, comment string) {
	if _, ok := config.values[key]; ok {
		config.values[key] = val
		if comment != "" {
			config.commentValues[key] = comment
		}
	} else {
		config.values[key] = val
		if comment != "" {
			config.commentValues[key] = comment
		}
		config.keys = append(config.keys, key)
	}
}

func (config *Config) Set(key, val, comment string) {
	config.configSection.set(key, val, comment)
}

func (config *Config) SectionSet(section, key, val, comment string) {
	if section == "" {
		config.set(key, val, comment)
	} else if sec, ok := config.sections[section]; ok {
		sec.set(key, val, comment)
	} else {
		sec = new(configSection)
		sec.name = section
		sec.values = make(map[string]string)
		sec.values[key] = val
		sec.commentValues = make(map[string]string)
		if comment != "" {
			sec.commentValues[key] = comment
		}
		sec.keys = make([]string, 1)
		sec.keys[0] = key
		config.sections[section] = sec
		config.sectionkeys = append(config.sectionkeys, section)
	}
}

func (config *Config) DelKey(key string) {
	delete(config.values, key)
	delete(config.commentValues, key)
	for i, v := range config.keys {
		if v == key {
			config.keys = append(config.keys[:i], config.keys[i+1:]...)
			break
		}
	}
}

func (config *Config) DelSectionKey(section, key string) {
	if section == "" {
		config.DelKey(key)
	} else if sec, ok := config.sections[section]; ok {
		delete(sec.values, key)
		delete(sec.commentValues, key)
		for i, v := range sec.keys {
			if v == key {
				sec.keys = append(sec.keys[:i], sec.keys[i+1:]...)
				break
			}
		}
	}
}

func (config *Config) DelSection(section string) {
	delete(config.sections, section)
}

func (config *Config) String(key string, def string) string {
	return config.string(config.values, key, def)
}
func (config *Config) Boolean(key string, def bool) bool {
	return config.boolean(config.values, key, def)
}
func (config *Config) Integer(key string, def int64) int64 {
	return config.integer(config.values, key, def)
}
func (config *Config) Float(key string, def float64) float64 {
	return config.float(config.values, key, def)
}
func (config *Config) StringSection(sec string, key string, def string) string {
	m, ok := config.sections[sec]
	if !ok {
		return def
	}
	return config.string(m.values, key, def)
}
func (config *Config) BooleanSection(sec string, key string, def bool) bool {
	m, ok := config.sections[sec]
	if !ok {
		return def
	}
	return config.boolean(m.values, key, def)
}
func (config *Config) IntegerSection(sec string, key string, def int64) int64 {
	m, ok := config.sections[sec]
	if !ok {
		return def
	}
	return config.integer(m.values, key, def)
}
func (config *Config) FloatSection(sec string, key string, def float64) float64 {
	m, ok := config.sections[sec]
	if !ok {
		return def
	}
	return config.float(m.values, key, def)
}

func (config *Config) Section(sec string) map[string]string {
	m, ok := config.sections[sec]
	if !ok {
		return nil
	}
	return m.values
}

type configSection struct {
	name   string
	values map[string]string

	commentSection string
	commentValues  map[string]string

	keys []string
}

type Config struct {
	configSection
	sections    map[string]*configSection
	sectionkeys []string
	path        string
}

func trimSpaceAndComment(sLine string) (line, comment string) {
	sLine = strings.TrimSpace(sLine)
	if len(sLine) == 0 {
		return "", ""
	}
	lineRune := []rune(sLine)
	sT := string(lineRune[0:1])
	if sT == "#" || sT == ";" {
		return "", strings.TrimSpace(string(lineRune[1:]))
	}

	line = sLine
	if sT == "[" {
		idx := strings.LastIndex(sLine, ";")
		if idx == -1 {
			idx = strings.LastIndex(sLine, "#")
		}
		if idx > 0 {
			comment = sLine[idx+1:]
			comment = strings.TrimSpace(comment)
			line = sLine[0:idx]
			line = strings.TrimSpace(line)
		}
	}
	return line, comment
}

func trimQuote(value string) string {
	if len(value) < 2 {
		return value
	}
	if value[0] == '"' && value[len(value)-1] == '"' && value[len(value)-2] != '\\' {
		value = value[1 : len(value)-1]
	} else if value[0] == '\'' && value[len(value)-1] == '\'' && value[len(value)-2] != '\\' {
		value = value[1 : len(value)-1]
	}
	return value
}
func trimValue(value string) (line, comment string) {
	value = strings.TrimSpace(value)
	nv := trimQuote(value)
	if len(nv) != len(value) {
		return nv, ""
	}

	idx := strings.LastIndex(value, ";")
	if idx == -1 {
		idx = strings.LastIndex(value, "#")
	}
	if idx > 0 {
		comment = value[idx+1:]
		comment = strings.TrimSpace(comment)
		value = value[0:idx]
		value = strings.TrimSpace(value)

		value = trimQuote(value)
	}

	return value, comment
}

func (config *Config) readIniFile(input io.Reader) error {
	config.values = make(map[string]string)
	config.sections = make(map[string]*configSection)
	config.commentValues = make(map[string]string)
	config.keys = make([]string, 0)
	config.sectionkeys = make([]string, 0)

	ln := 0
	var comment string
	var currentSection *configSection
	scanner := bufio.NewScanner(input)
	for scanner.Scan() {
		ln++
		curLine := scanner.Text()
		if ln == 1 && len(curLine) > 0 { //UTF-8(BOM) file begin with EE,BB,BF
			lineRune := []rune(curLine)
			lnr := len(lineRune)
			if lnr > 0 && lineRune[0] == 65279 {
				lineRune = lineRune[1:lnr]
			}
			curLine = string(lineRune)
		}
		var curComment string
		curLine, curComment = trimSpaceAndComment(curLine)
		if curComment != "" {
			comment += " " + curComment
		}
		if len(curLine) == 0 {
			continue
		}

		if strings.HasPrefix(curLine, "[") {
			if !strings.HasSuffix(curLine, "]") {
				return fmt.Errorf("begin with '[' but not end with ']';line[%d]", ln)
			}
			sectionName := curLine[1 : len(curLine)-1]

			if sect, ok := config.sections[sectionName]; !ok {
				currentSection = new(configSection)
				currentSection.name = sectionName
				currentSection.values = make(map[string]string)
				currentSection.commentSection = comment
				currentSection.commentValues = make(map[string]string)
				currentSection.keys = make([]string, 0)
				config.sections[currentSection.name] = currentSection
				config.sectionkeys = append(config.sectionkeys, sectionName)
			} else {
				currentSection = sect
			}

			comment = ""
			continue
		}

		index := strings.Index(curLine, "=")

		if index <= 0 {
			return fmt.Errorf("requires an equals between the key and value;line[%d]", ln)
		}

		key := strings.TrimSpace(curLine[0:index])
		value := strings.TrimSpace(curLine[index+1:])
		nv, co := trimValue(value)
		value = nv
		if len(co) > 0 {
			comment += " " + co
		}

		if currentSection != nil {
			currentSection.values[key] = value
			currentSection.commentValues[key] = comment
			currentSection.keys = append(currentSection.keys, key)
		} else {
			config.values[key] = value
			config.commentValues[key] = comment
			config.keys = append(config.keys, key)
		}
		comment = ""
	}

	return scanner.Err()
}

func (config *Config) getVal(kv map[string]string, key string) (string, bool) {
	v, ok := kv[key]
	return v, ok
}

func (config *Config) string(kv map[string]string, key string, def string) string {
	v, ok := config.getVal(kv, key)
	if !ok {
		return def
	}
	return v
}
func (config *Config) boolean(kv map[string]string, key string, def bool) bool {
	v, ok := config.getVal(kv, key)
	if !ok {
		return def
	}
	b, e := strconv.ParseBool(v)
	if e != nil {
		return def
	}
	return b
}
func (config *Config) integer(kv map[string]string, key string, def int64) int64 {
	v, ok := config.getVal(kv, key)
	if !ok {
		return def
	}
	i, e := strconv.ParseInt(v, 0, 64)
	if e != nil {
		return def
	}
	return i
}
func (config *Config) float(kv map[string]string, key string, def float64) float64 {
	v, ok := config.getVal(kv, key)
	if !ok {
		return def
	}
	f, e := strconv.ParseFloat(v, 64)
	if e != nil {
		return def
	}
	return f
}
