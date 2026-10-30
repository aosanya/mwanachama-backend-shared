package spec

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

func (f Field) defaultLiteral() (string, error) {
	switch f.Type {
	case TypeInt:
		n, err := strconv.ParseInt(f.Default, 10, 64)
		if err != nil {
			return "", fmt.Errorf("%q is not a whole number", f.Default)
		}
		return strconv.FormatInt(n, 10), nil
	case TypeFloat:
		n, err := strconv.ParseFloat(f.Default, 64)
		if err != nil || math.IsNaN(n) || math.IsInf(n, 0) {
			return "", fmt.Errorf("%q is not a finite number", f.Default)
		}
		return strconv.FormatFloat(n, 'f', -1, 64), nil
	case TypeBool:
		b, err := strconv.ParseBool(f.Default)
		if err != nil {
			return "", fmt.Errorf("%q is not true or false", f.Default)
		}
		return strconv.FormatBool(b), nil
	default:
		return quotedLiteral(f.Default), nil
	}
}

func quotedLiteral(text string) string {
	return "'" + strings.ReplaceAll(text, "'", "''") + "'"
}

func (s *Spec) defaultProblems() []string {
	var problems []string
	for _, o := range s.Objects {
		for _, f := range o.Fields {
			if f.Default == "" {
				continue
			}
			if _, err := f.defaultLiteral(); err != nil {
				problems = append(problems, fmt.Sprintf("object %q: field %q is %s and its default %v", o.Name, f.Name, f.Type, err))
			}
		}
	}
	return problems
}
