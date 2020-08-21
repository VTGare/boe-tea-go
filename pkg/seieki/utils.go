package seieki

import (
	"fmt"
	"strings"
)

func beautifyPixiv(url string) string {
	if strings.HasPrefix(url, "https://www.pixiv.net/member_illust.php?mode=medium&illust_id=") {
		id := strings.TrimPrefix(url, "https://www.pixiv.net/member_illust.php?mode=medium&illust_id=")
		url = fmt.Sprintf("https://www.pixiv.net/en/artworks/%v", id)
	}

	return url
}
