package main

type Result struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func (er *Result) Error() string {
	return er.Message
}
