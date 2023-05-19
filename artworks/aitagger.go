package artworks

import (
	"strings"
)

type AITagger struct{}

func (ai AITagger) AITag(tags []string) bool {
	aiTags := []string{
		"aiart",
		"aigenerated",
		"aiイラスト",
		"createdwithai",
		"dall-e",
		"midjourney",
		"stablediffusion",
	}

	for _, tag := range tags {
		for _, test := range aiTags {
			if strings.Contains(strings.ToLower(tag), test) {
				return true
			}
		}
	}
	return false
}

// for _, tag := range tags {
// 	aiGenerated := arrays.AnyFunc(aiTags, func(aiTag string) bool {
// 		return strings.Contains(strings.ToLower(tag), aiTag)
// 	})
// 	if aiGenerated {
// 		break
// 	}
// }
