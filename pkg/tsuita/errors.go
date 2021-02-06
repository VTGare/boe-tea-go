package tsuita

import "fmt"

//Declarations of Tsuita errors
var (
	ErrBadURL           = fmt.Errorf("Provided URL is not a Tweet")
	ErrRateLimitReached = fmt.Errorf("Too Many Requests. Try again later")
)
