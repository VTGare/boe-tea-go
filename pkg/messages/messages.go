package messages

func FormatBool(b bool) string {
	if b {
		return "enabled"
	}

	return "disabled"
}
