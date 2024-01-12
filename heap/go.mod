module github.com/vahagz/disk-allocator/heap

go 1.19

replace (
	github.com/vahagz/pager v0.0.1 => ../pkg/rbtree/pkg/pager
	github.com/vahagz/rbtree v0.0.1 => ../pkg/rbtree
)

require (
	github.com/pkg/errors v0.9.1
	github.com/vahagz/pager v0.0.1
	github.com/vahagz/rbtree v0.0.1
)

require (
	github.com/edsrzf/mmap-go v1.1.0 // indirect
	golang.org/x/sys v0.0.0-20211216021012-1d35b9e2eb4e // indirect
)
