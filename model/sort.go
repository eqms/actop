package model

import "sort"

type SortField string

const (
	SortName   SortField = "name"
	SortState  SortField = "state"
	SortCPU    SortField = "cpu"
	SortMem    SortField = "mem"
	SortNet    SortField = "net"
	SortIO     SortField = "io"
	SortPIDs   SortField = "pids"
	SortUptime SortField = "uptime"
)

// SortFields is every sort field, in the order the sort menu lists them.
var SortFields = []SortField{SortName, SortState, SortCPU, SortMem, SortNet, SortIO, SortPIDs, SortUptime}

func sortContainers(cs []*Container, field SortField, reversed bool) {
	less := lessFn(field)
	sort.SliceStable(cs, func(i, j int) bool {
		if reversed {
			i, j = j, i
		}
		return less(cs[i], cs[j])
	})
}

func lessFn(field SortField) func(a, b *Container) bool {
	switch field {
	case SortState:
		return func(a, b *Container) bool {
			if a.State != b.State {
				return a.State < b.State
			}
			return a.ID < b.ID
		}
	case SortCPU:
		return func(a, b *Container) bool { return a.Sample.CPUPercent > b.Sample.CPUPercent }
	case SortMem:
		return func(a, b *Container) bool { return a.Sample.MemUsage > b.Sample.MemUsage }
	case SortNet:
		return func(a, b *Container) bool {
			return a.Sample.NetRxRate+a.Sample.NetTxRate > b.Sample.NetRxRate+b.Sample.NetTxRate
		}
	case SortIO:
		return func(a, b *Container) bool {
			return a.Sample.IOReadRate+a.Sample.IOWriteRate > b.Sample.IOReadRate+b.Sample.IOWriteRate
		}
	case SortPIDs:
		return func(a, b *Container) bool { return a.Sample.PIDs > b.Sample.PIDs }
	case SortUptime:
		return func(a, b *Container) bool { return a.Started.After(b.Started) }
	default: // SortName
		return func(a, b *Container) bool { return a.ID < b.ID }
	}
}
