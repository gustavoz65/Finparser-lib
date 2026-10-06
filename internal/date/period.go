package date

import (
	"regexp"
	"time"

	"github.com/gustavoz65/finparser-lib/internal/textnorm"
)

const datePat = `(\d{1,2}/\d{1,2}/\d{2,4}|\d{1,2}(?: de)? [a-z]{3,9}\.?(?: de)? \d{4})`

var periodRe = regexp.MustCompile(datePat + `\s*(?:a|ate|-|ao|à)\s*` + datePat)

// FindPeriod procura no texto um intervalo como "01/03/2026 a 31/03/2026" ou
// "01 de março de 2026 até 31 de março de 2026". Só aceita datas com ano.
func FindPeriod(text string) (start, end time.Time, ok bool) {
	k := textnorm.Key(text)
	for _, m := range periodRe.FindAllStringSubmatch(k, -1) {
		a, err1 := Parse(m[1])
		b, err2 := Parse(m[2])
		if err1 != nil || err2 != nil || !a.HasYear || !b.HasYear {
			continue
		}
		if b.Time().Before(a.Time()) {
			continue
		}
		return a.Time(), b.Time(), true
	}
	return time.Time{}, time.Time{}, false
}
