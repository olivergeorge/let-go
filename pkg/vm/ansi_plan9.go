//go:build plan9

/*
 * Copyright (c) 2021-2026 Marcin Gasperowicz <xnooga@gmail.com>
 * SPDX-License-Identifier: MIT
 */

package vm

// rio doesn't render ANSI escape sequences — strip them from error output.
const (
	ansiBold     = ""
	ansiBoldRed  = ""
	ansiBoldBlue = ""
	ansiReset    = ""
)
