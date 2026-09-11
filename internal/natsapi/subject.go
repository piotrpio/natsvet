// Copyright 2026 Synadia Communications Inc.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package natsapi

import (
	"strings"
	"unicode/utf8"
)

// Subject functions ported from nats-server server/sublist.go. The control
// flow follows the server line for line so a diff against it stays
// mechanical; the tests run the server's own tables.

const (
	pwc   = '*'
	fwc   = '>'
	tsep  = "."
	btsep = '.'
)

// IsValidSubject reports whether subject is a valid NATS subject: non-empty
// tokens, no whitespace, no embedded NUL or invalid UTF-8, and a full
// wildcard only as the last token.
func IsValidSubject(subject string) bool {
	if subject == "" {
		return false
	}
	if strings.IndexByte(subject, 0) >= 0 {
		return false
	}
	for _, r := range subject {
		if r == utf8.RuneError {
			return false
		}
	}
	sfwc := false
	for t := range strings.SplitSeq(subject, tsep) {
		length := len(t)
		if length == 0 || sfwc {
			return false
		}
		if length > 1 {
			if strings.ContainsAny(t, "\t\n\f\r ") {
				return false
			}
			continue
		}
		switch t[0] {
		case fwc:
			sfwc = true
		case ' ', '\t', '\n', '\r', '\f':
			return false
		}
	}
	return true
}

// SubjectIsLiteral reports whether subject has no wildcard token.
func SubjectIsLiteral(subject string) bool {
	for i, c := range subject {
		if c == pwc || c == fwc {
			if (i == 0 || subject[i-1] == btsep) &&
				(i+1 == len(subject) || subject[i+1] == btsep) {
				return false
			}
		}
	}
	return true
}

func tokenizeSubjectIntoSlice(tts []string, subject string) []string {
	start := 0
	for i := range len(subject) {
		if subject[i] == btsep {
			tts = append(tts, subject[start:i])
			start = i + 1
		}
	}
	tts = append(tts, subject[start:])
	return tts
}

func analyzeTokens(tokens []string) (hasPWC, hasFWC bool) {
	for _, t := range tokens {
		if lt := len(t); lt == 0 || lt > 1 {
			continue
		}
		switch t[0] {
		case pwc:
			hasPWC = true
		case fwc:
			hasFWC = true
		}
	}
	return
}

func tokensCanMatch(t1, t2 string) bool {
	if len(t1) == 0 || len(t2) == 0 {
		return false
	}
	t1c, t2c := t1[0], t2[0]
	if t1c == pwc || t2c == pwc || t1c == fwc || t2c == fwc {
		return true
	}
	return t1 == t2
}

// SubjectsCollide reports whether two subjects could both match a single
// literal subject.
func SubjectsCollide(subj1, subj2 string) bool {
	if subj1 == subj2 {
		return true
	}
	tsa, tsb := [32]string{}, [32]string{}
	toks1 := tokenizeSubjectIntoSlice(tsa[:0], subj1)
	toks2 := tokenizeSubjectIntoSlice(tsb[:0], subj2)
	pwc1, fwc1 := analyzeTokens(toks1)
	pwc2, fwc2 := analyzeTokens(toks2)
	l1, l2 := !(pwc1 || fwc1), !(pwc2 || fwc2)
	if l1 && l2 {
		return subj1 == subj2
	}
	if l1 && !l2 {
		return isSubsetMatchTokenized(toks1, toks2)
	} else if l2 && !l1 {
		return isSubsetMatchTokenized(toks2, toks1)
	}
	if !fwc1 && !fwc2 && len(toks1) != len(toks2) {
		return false
	}
	if lt1, lt2 := len(toks1), len(toks2); lt1 != lt2 {
		if lt1 < lt2 && !fwc1 || lt2 < lt1 && !fwc2 {
			return false
		}
	}
	stop := min(len(toks1), len(toks2))
	for i := range stop {
		if !tokensCanMatch(toks1[i], toks2[i]) {
			return false
		}
	}
	return true
}

// SubjectIsSubsetMatch reports whether every literal subject matched by
// subject is also matched by test. Both may contain wildcards.
func SubjectIsSubsetMatch(subject, test string) bool {
	tsa := [32]string{}
	tts := tokenizeSubjectIntoSlice(tsa[:0], subject)
	return isSubsetMatch(tts, test)
}

func isSubsetMatch(tokens []string, test string) bool {
	tsa := [32]string{}
	tts := tokenizeSubjectIntoSlice(tsa[:0], test)
	return isSubsetMatchTokenized(tokens, tts)
}

func isSubsetMatchTokenized(tokens, test []string) bool {
	for i, t2 := range test {
		if i >= len(tokens) {
			return false
		}
		l := len(t2)
		if l == 0 {
			return false
		}
		if t2[0] == fwc && l == 1 {
			return true
		}
		t1 := tokens[i]
		l = len(t1)
		if l == 0 || t1[0] == fwc && l == 1 {
			return false
		}
		if t1[0] == pwc && len(t1) == 1 {
			m := t2[0] == pwc && len(t2) == 1
			if !m {
				return false
			}
			if i >= len(test) {
				return true
			}
			continue
		}
		if t2[0] != pwc && strings.Compare(t1, t2) != 0 {
			return false
		}
	}
	return len(tokens) == len(test)
}
