package allocator

import "github.com/vahagz/pager"

type Options struct {
	TargetPageSize uint16
	TreePageSize   uint16
	Pager          *pager.Pager
}
