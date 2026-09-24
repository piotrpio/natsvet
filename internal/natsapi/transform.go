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
	"errors"
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
)

// Subject transform validation ported from nats-server: ValidateMapping
// from server/sublist.go, and the error paths of the non-strict
// NewSubjectTransformWithStrict with indexPlaceHolders from
// server/subject_transform.go. Only the decision whether a mapping is
// accepted is ported, not the transform itself; control flow, expressions
// and error texts follow the server so a diff against it stays mechanical.

var (
	errBadSubject                        = errors.New("invalid subject")
	errInvalidMappingDestination         = errors.New("invalid mapping destination")
	errInvalidMappingDestinationSubject  = fmt.Errorf("%w: invalid transform", errInvalidMappingDestination)
	errUnknownMappingDestinationFunction = fmt.Errorf("%w: unknown function", errInvalidMappingDestination)
	errMappingDestinationIndexOutOfRange = fmt.Errorf("%w: wildcard index out of range", errInvalidMappingDestination)
	errMappingDestinationNotEnoughArgs   = fmt.Errorf("%w: not enough arguments passed to the function", errInvalidMappingDestination)
	errMappingDestinationInvalidArg      = fmt.Errorf("%w: function argument is invalid or in the wrong format", errInvalidMappingDestination)
	errMappingDestinationTooManyArgs     = fmt.Errorf("%w: too many arguments passed to the function", errInvalidMappingDestination)
)

type mappingDestinationErr struct {
	token string
	err   error
}

func (e *mappingDestinationErr) Error() string {
	if e.token == "" {
		return e.err.Error()
	}
	return fmt.Sprintf("%s in %s", e.err, e.token)
}

func (e *mappingDestinationErr) Is(target error) bool {
	return target == errInvalidMappingDestination
}

var (
	commaSeparatorRegEx                = regexp.MustCompile(`,\s*`)
	partitionMappingFunctionRegEx      = regexp.MustCompile(`{{\s*[pP]artition\s*\((.*)\)\s*}}`)
	wildcardMappingFunctionRegEx       = regexp.MustCompile(`{{\s*[wW]ildcard\s*\((.*)\)\s*}}`)
	splitFromLeftMappingFunctionRegEx  = regexp.MustCompile(`{{\s*[sS]plit[fF]rom[lL]eft\s*\((.*)\)\s*}}`)
	splitFromRightMappingFunctionRegEx = regexp.MustCompile(`{{\s*[sS]plit[fF]rom[rR]ight\s*\((.*)\)\s*}}`)
	sliceFromLeftMappingFunctionRegEx  = regexp.MustCompile(`{{\s*[sS]lice[fF]rom[lL]eft\s*\((.*)\)\s*}}`)
	sliceFromRightMappingFunctionRegEx = regexp.MustCompile(`{{\s*[sS]lice[fF]rom[rR]ight\s*\((.*)\)\s*}}`)
	splitMappingFunctionRegEx          = regexp.MustCompile(`{{\s*[sS]plit\s*\((.*)\)\s*}}`)
	leftMappingFunctionRegEx           = regexp.MustCompile(`{{\s*[lL]eft\s*\((.*)\)\s*}}`)
	rightMappingFunctionRegEx          = regexp.MustCompile(`{{\s*[rR]ight\s*\((.*)\)\s*}}`)
	randomMappingFunctionRegEx         = regexp.MustCompile(`{{\s*[rR]andom\s*\((.*)\)\s*}}`)
)

type transformType int

const (
	noTransform transformType = iota
	badTransform
	partitionTransform
	wildcardTransform
	splitFromLeftTransform
	splitFromRightTransform
	sliceFromLeftTransform
	sliceFromRightTransform
	splitTransform
	leftTransform
	rightTransform
	randomTransform
)

// ValidateMapping returns the error nats-server reports for a subject
// transform from src to dest, or nil when the server accepts it. An empty
// dest is valid; an empty src means ">".
func ValidateMapping(src, dest string) error {
	if dest == "" {
		return nil
	}
	sfwc := false
	for t := range strings.SplitSeq(dest, tsep) {
		length := len(t)
		if length == 0 || sfwc {
			return &mappingDestinationErr{t, errInvalidMappingDestinationSubject}
		}
		if length > 4 && t[0] == '{' && t[1] == '{' && t[length-2] == '}' && t[length-1] == '}' {
			if !partitionMappingFunctionRegEx.MatchString(t) &&
				!wildcardMappingFunctionRegEx.MatchString(t) &&
				!splitFromLeftMappingFunctionRegEx.MatchString(t) &&
				!splitFromRightMappingFunctionRegEx.MatchString(t) &&
				!sliceFromLeftMappingFunctionRegEx.MatchString(t) &&
				!sliceFromRightMappingFunctionRegEx.MatchString(t) &&
				!splitMappingFunctionRegEx.MatchString(t) &&
				!leftMappingFunctionRegEx.MatchString(t) &&
				!rightMappingFunctionRegEx.MatchString(t) &&
				!randomMappingFunctionRegEx.MatchString(t) {
				return &mappingDestinationErr{t, errUnknownMappingDestinationFunction}
			}
			continue
		}
		if length == 1 && t[0] == fwc {
			sfwc = true
		} else if strings.ContainsAny(t, "\t\n\f\r ") {
			return errInvalidMappingDestinationSubject
		}
	}
	return SubjectTransformErr(src, dest)
}

// SubjectTransformErr returns the error nats-server's non-strict
// NewSubjectTransform reports for src and dest, or nil when it builds the
// transform. The server validates RePublish with it directly and ends
// ValidateMapping with it.
func SubjectTransformErr(src, dest string) error {
	if dest == "" {
		return nil
	}
	if src == "" {
		src = string(fwc)
	}
	sv, _, npwcs, hasFwc := subjectInfo(src)
	dv, dtokens, dnpwcs, dHasFwc := subjectInfo(dest)
	if !sv || !dv || dnpwcs > 0 || hasFwc != dHasFwc {
		return errBadSubject
	}
	if npwcs > 0 || hasFwc {
		for _, token := range dtokens {
			kind, indexes, err := indexPlaceHolders(token)
			if err != nil {
				return err
			}
			if kind == noTransform || kind == randomTransform {
				continue
			}
			for _, wildcardIndex := range indexes {
				if wildcardIndex > npwcs {
					return &mappingDestinationErr{fmt.Sprintf("%s: [%d]", token, wildcardIndex), errMappingDestinationIndexOutOfRange}
				}
			}
		}
		return nil
	}
	for _, token := range dtokens {
		kind, _, err := indexPlaceHolders(token)
		if err != nil {
			return err
		}
		if kind != noTransform && kind != randomTransform && kind != partitionTransform {
			return &mappingDestinationErr{token, errMappingDestinationIndexOutOfRange}
		}
	}
	return nil
}

func getMappingFunctionArgs(functionRegEx *regexp.Regexp, token string) []string {
	commandStrings := functionRegEx.FindStringSubmatch(token)
	if len(commandStrings) > 1 {
		return commaSeparatorRegEx.Split(commandStrings[1], -1)
	}
	return nil
}

func transformIndexIntArgsHelper(token string, args []string, kind transformType) (transformType, []int, error) {
	if len(args) < 2 {
		return badTransform, nil, &mappingDestinationErr{token, errMappingDestinationNotEnoughArgs}
	}
	if len(args) > 2 {
		return badTransform, nil, &mappingDestinationErr{token, errMappingDestinationTooManyArgs}
	}
	i, err := strconv.Atoi(strings.Trim(args[0], " "))
	if err != nil {
		return badTransform, nil, &mappingDestinationErr{token, errMappingDestinationInvalidArg}
	}
	if _, err := strconv.ParseInt(strings.Trim(args[1], " "), 10, 32); err != nil {
		return badTransform, nil, &mappingDestinationErr{token, errMappingDestinationInvalidArg}
	}
	return kind, []int{i}, nil
}

// indexPlaceHolders classifies one destination token and returns the
// source wildcard indexes it refers to.
func indexPlaceHolders(token string) (transformType, []int, error) {
	length := len(token)
	if length <= 1 {
		return noTransform, nil, nil
	}
	if token[0] == '$' {
		tp, err := strconv.Atoi(token[1:])
		if err != nil {
			return noTransform, nil, nil
		}
		return wildcardTransform, []int{tp}, nil
	}
	if length <= 4 || token[0] != '{' || token[1] != '{' || token[length-2] != '}' || token[length-1] != '}' {
		return noTransform, nil, nil
	}

	if args := getMappingFunctionArgs(wildcardMappingFunctionRegEx, token); args != nil {
		if len(args) == 1 && args[0] == "" {
			return badTransform, nil, &mappingDestinationErr{token, errMappingDestinationNotEnoughArgs}
		}
		if len(args) > 1 {
			return badTransform, nil, &mappingDestinationErr{token, errMappingDestinationTooManyArgs}
		}
		tokenIndex, err := strconv.Atoi(strings.Trim(args[0], " "))
		if err != nil {
			return badTransform, nil, &mappingDestinationErr{token, errMappingDestinationInvalidArg}
		}
		return wildcardTransform, []int{tokenIndex}, nil
	}

	if args := getMappingFunctionArgs(partitionMappingFunctionRegEx, token); args != nil {
		if len(args) < 1 {
			return badTransform, nil, &mappingDestinationErr{token, errMappingDestinationNotEnoughArgs}
		}
		n, err := strconv.Atoi(strings.Trim(args[0], " "))
		if err != nil || n > math.MaxInt32 {
			return badTransform, nil, &mappingDestinationErr{token, errMappingDestinationInvalidArg}
		}
		tokenIndexes := make([]int, len(args[1:]))
		for ti, t := range args[1:] {
			i, err := strconv.Atoi(strings.Trim(t, " "))
			if err != nil {
				return badTransform, nil, &mappingDestinationErr{token, errMappingDestinationInvalidArg}
			}
			tokenIndexes[ti] = i
		}
		return partitionTransform, tokenIndexes, nil
	}

	for _, f := range []struct {
		re   *regexp.Regexp
		kind transformType
	}{
		{splitFromLeftMappingFunctionRegEx, splitFromLeftTransform},
		{splitFromRightMappingFunctionRegEx, splitFromRightTransform},
		{sliceFromLeftMappingFunctionRegEx, sliceFromLeftTransform},
		{sliceFromRightMappingFunctionRegEx, sliceFromRightTransform},
		{rightMappingFunctionRegEx, rightTransform},
		{leftMappingFunctionRegEx, leftTransform},
	} {
		if args := getMappingFunctionArgs(f.re, token); args != nil {
			return transformIndexIntArgsHelper(token, args, f.kind)
		}
	}

	if args := getMappingFunctionArgs(splitMappingFunctionRegEx, token); args != nil {
		if len(args) < 2 {
			return badTransform, nil, &mappingDestinationErr{token, errMappingDestinationNotEnoughArgs}
		}
		if len(args) > 2 {
			return badTransform, nil, &mappingDestinationErr{token, errMappingDestinationTooManyArgs}
		}
		i, err := strconv.Atoi(strings.Trim(args[0], " "))
		if err != nil {
			return badTransform, nil, &mappingDestinationErr{token, errMappingDestinationInvalidArg}
		}
		if strings.Contains(args[1], " ") || strings.Contains(args[1], tsep) {
			return badTransform, nil, &mappingDestinationErr{token, errMappingDestinationInvalidArg}
		}
		return splitTransform, []int{i}, nil
	}

	if args := getMappingFunctionArgs(randomMappingFunctionRegEx, token); args != nil {
		if len(args) != 1 {
			return badTransform, nil, &mappingDestinationErr{token, errMappingDestinationNotEnoughArgs}
		}
		n, err := strconv.Atoi(strings.Trim(args[0], " "))
		if err != nil || n > math.MaxInt32 {
			return badTransform, nil, &mappingDestinationErr{token, errMappingDestinationInvalidArg}
		}
		return randomTransform, nil, nil
	}

	return badTransform, nil, &mappingDestinationErr{token, errUnknownMappingDestinationFunction}
}

// subjectInfo reports whether subject is valid for a transform, its tokens,
// its number of partial wildcards and whether it ends in a full wildcard.
func subjectInfo(subject string) (bool, []string, int, bool) {
	if subject == "" {
		return false, nil, 0, false
	}
	npwcs := 0
	sfwc := false
	tokens := strings.Split(subject, tsep)
	for _, t := range tokens {
		if len(t) == 0 || sfwc {
			return false, nil, 0, false
		}
		if len(t) > 1 {
			continue
		}
		switch t[0] {
		case fwc:
			sfwc = true
		case pwc:
			npwcs++
		}
	}
	return true, tokens, npwcs, sfwc
}
