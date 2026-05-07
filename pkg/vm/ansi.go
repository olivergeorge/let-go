//go:build !plan9

/*
 * Copyright (c) 2021-2026 Marcin Gasperowicz <xnooga@gmail.com>
 * SPDX-License-Identifier: MIT
 */

package vm

// ANSI escape sequences used by error formatting. On platforms without ANSI
// support (plan9 / rio) these are stubbed out to empty strings — see
// ansi_plan9.go.
const (
	ansiBold     = "[1m"
	ansiBoldRed  = "[1;31m"
	ansiBoldBlue = "[1;34m"
	ansiReset    = "[0m"
)
