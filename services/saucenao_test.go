package services

import "testing"

func TestSauceNao(t *testing.T) {
	res, err := SearchByURL("https%3A%2F%2Fimages-ext-1.discordapp.net%2Fexternal%2FlPAq5wxKWxDNO358Ea9fDrjBjfW5Kl02BuoFEE8mrZY%2Fhttps%2Fpbs.twimg.com%2Fmedia%2FEVy0c0CVAAAeEgb.jpg%253Alarge%3Fwidth%3D291%26height%3D441")
	if err != nil {
		t.Log(err)
		t.FailNow()
	}

	t.Log(res)
	t.FailNow()
}
