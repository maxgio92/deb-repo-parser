package deb

func unique(ss []string) []string {
	keys := make(map[string]bool)

	unique := []string{}
	for _, v := range ss {
		if ok := keys[v]; !ok {
			keys[v] = true
			unique = append(unique, v)
		}
	}

	return unique
}
