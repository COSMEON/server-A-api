package main

type Codebase struct {
	UUID string `json:"uuid"`
	Name string `json:"name"`
}

type File struct {
	Path string `json:"path"`
	Name string `json:"name"`
	Size int64  `json:"size"`
}
