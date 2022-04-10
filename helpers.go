package main

import (
	"strconv"
	"fmt"
)

func FirstN(s string, n uint) string {
	if uint(len(s)) > n {
		return s[:n]
	}
	return s
}

func LastN(s string, n uint) string {
	if uint(len(s)) > n {
		return s[uint(len(s)) - n:]
	}
	return s
}

func MinUint(x, y uint) uint {
	if x < y {
		return x
	}
	return y
}

func MaxInt(x, y int) int {
	if x > y {
		return x
	}
	return y
}

func FormatSecondsToMinutes(s string) (string) {
	temp, err := strconv.Atoi(s)
	if err != nil {
		return ""
	}
	if temp / 60 >= 60 {
		return fmt.Sprintf("%v:%02v:%02v", (temp / 60 / 60), (temp / 60) % 60, temp % 60)
	}
	return fmt.Sprintf("%v:%02v", temp / 60, temp % 60)
}