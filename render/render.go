package render

type Graph struct {
	NodeSet []*Node `json:"nodes"`
	EdgeSet []*Edge `json:"links"`
}

type Node struct {
	ID     int    `json:"id"`
	Set    int    `json:"group"`
	Name   string `json:"name"`
	Detail string `json:"detail"`
}

type Edge struct {
	From   int    `json:"source"`
	To     int    `json:"target"`
	Name   string `json:"name"`
	Detail string `json:"detail"`
}
