package bufferhook

import (
	"bytes"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
)

var quotingRequired = regexp.MustCompile(`[^a-zA-Z0-9_/@^+.-]`)

// BufferHook is a logrus hook to cache log messages into a buffer to
// be used in reporting later on. As bytes.Buffer is included in this
// struct all methods of it can be used.
type BufferHook struct {
	levels []logrus.Level

	bytes.Buffer
}

// New creates a new BufferHook instance
func New(level logrus.Level) *BufferHook {
	levels := []logrus.Level{}
	for _, l := range []logrus.Level{
		logrus.PanicLevel,
		logrus.FatalLevel,
		logrus.ErrorLevel,
		logrus.WarnLevel,
		logrus.InfoLevel,
		logrus.DebugLevel,
	} {
		if l <= level {
			levels = append(levels, l)
		}
	}

	return &BufferHook{
		levels: levels,
	}
}

// Levels returns the enabled levels for this hook (interface logrus.Hook)
func (b BufferHook) Levels() []logrus.Level { return b.levels }

// Fire retrieves the event and generates the log line (interface logrus.Hook)
func (b BufferHook) Fire(e *logrus.Entry) error {
	_, err := b.Write(b.formatLine(e))
	return err
}

func (b BufferHook) formatLine(entry *logrus.Entry) []byte {
	buf := new(bytes.Buffer)

	levelText := strings.ToUpper(entry.Level.String())[0:4]
	fmt.Fprintf(buf, "%s[%s] %-44s ", levelText, entry.Time.Format(time.RFC3339), entry.Message)

	keys := []string{}
	for k := range entry.Data {
		keys = append(keys, k)
	}

	sort.Strings(keys)

	for _, k := range keys {
		v := entry.Data[k]
		fmt.Fprintf(buf, " %s=", k)
		b.appendValue(buf, v)
	}

	buf.Write([]byte{'\n'})
	return buf.Bytes()
}

func (b BufferHook) needsQuoting(text string) bool {
	return len(text) == 0 || quotingRequired.MatchString(text)
}

func (b BufferHook) appendValue(buf *bytes.Buffer, value interface{}) {
	stringVal, ok := value.(string)
	if !ok {
		stringVal = fmt.Sprint(value)
	}

	if !b.needsQuoting(stringVal) {
		buf.WriteString(stringVal)
	} else {
		buf.WriteString(fmt.Sprintf("%q", stringVal))
	}
}
