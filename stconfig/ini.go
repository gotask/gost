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
	return cfg, nil
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

type configSection struct {
	name   string
	values map[string]string
}

type Config struct {
	configSection
	sections map[string]*configSection
}

func trimSpaceAndComment(sLine string) string {
	sLine = strings.TrimSpace(sLine)
	if len(sLine) == 0 {
		return ""
	}
	lineRune := []rune(sLine)
	sT := string(lineRune[0:1])
	if sT == "#" || sT == ";" {
		return ""
	}
	return sLine
}

func (config *Config) readIniFile(input io.Reader) error {
	config.values = make(map[string]string)
	config.sections = make(map[string]*configSection)

	ln := 0
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
		curLine = trimSpaceAndComment(curLine)
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
				config.sections[currentSection.name] = currentSection
			} else {
				currentSection = sect
			}

			continue
		}

		index := strings.Index(curLine, "=")

		if index <= 0 {
			return fmt.Errorf("requires an equals between the key and value;line[%d]", ln)
		}

		key := strings.TrimSpace(curLine[0:index])
		value := strings.Trim(strings.TrimSpace(curLine[index+1:]), "\"'")

		if _, ok := config.values[key]; !ok {
			config.values[key] = value
		}
		if currentSection != nil {
			currentSection.values[key] = value
		}
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
