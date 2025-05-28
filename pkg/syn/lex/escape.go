package lex

var escapeMap = map[string]string{
	`\a`: "\a",
	`\b`: "\b",
	`\f`: "\f",
	`\n`: "\n",
	`\r`: "\r",
	`\t`: "\t",
	`\v`: "\v",
	`\"`: `"`,
}
