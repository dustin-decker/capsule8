// Copyright 2017 Capsule8, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package sensor

import (
	"fmt"
	"sync/atomic"

	api "github.com/capsule8/capsule8/api/v0"

	"github.com/capsule8/capsule8/pkg/expression"
	"github.com/capsule8/capsule8/pkg/sys"
	"github.com/capsule8/capsule8/pkg/sys/perf"

	"google.golang.org/genproto/googleapis/rpc/code"
)

var syscallEnterEventTypes = expression.FieldTypeMap{
	"id":   expression.ValueTypeSignedInt64,
	"arg0": expression.ValueTypeUnsignedInt64,
	"arg1": expression.ValueTypeUnsignedInt64,
	"arg2": expression.ValueTypeUnsignedInt64,
	"arg3": expression.ValueTypeUnsignedInt64,
	"arg4": expression.ValueTypeUnsignedInt64,
	"arg5": expression.ValueTypeUnsignedInt64,
}

var syscallExitEventTypes = expression.FieldTypeMap{
	"id":  expression.ValueTypeSignedInt64,
	"ret": expression.ValueTypeSignedInt64,
}

type syscallFilter struct {
	sensor *Sensor
}

func (f *syscallFilter) decodeDummySysEnter(sample *perf.SampleRecord, data perf.TraceEventSampleData) (interface{}, error) {
	return nil, nil
}

func (f *syscallFilter) decodeSyscallTraceEnter(sample *perf.SampleRecord, data perf.TraceEventSampleData) (interface{}, error) {
	ev := f.sensor.NewEventFromSample(sample, data)
	if ev == nil {
		return nil, nil
	}
	ev.Event = &api.TelemetryEvent_Syscall{
		Syscall: &api.SyscallEvent{
			Type: api.SyscallEventType_SYSCALL_EVENT_TYPE_ENTER,
			Id:   data["id"].(int64),
			Arg0: data["arg0"].(uint64),
			Arg1: data["arg1"].(uint64),
			Arg2: data["arg2"].(uint64),
			Arg3: data["arg3"].(uint64),
			Arg4: data["arg4"].(uint64),
			Arg5: data["arg5"].(uint64),
		},
	}

	return ev, nil
}

func (f *syscallFilter) decodeSysExit(sample *perf.SampleRecord, data perf.TraceEventSampleData) (interface{}, error) {
	ev := f.sensor.NewEventFromSample(sample, data)
	if ev == nil {
		return nil, nil
	}
	ev.Event = &api.TelemetryEvent_Syscall{
		Syscall: &api.SyscallEvent{
			Type: api.SyscallEventType_SYSCALL_EVENT_TYPE_EXIT,
			Id:   data["id"].(int64),
			Ret:  data["ret"].(int64),
		},
	}

	return ev, nil
}

func containsIDFilter(expr *api.Expression) bool {
	if expr == nil {
		return false
	}

	switch expr.GetType() {
	case api.Expression_LOGICAL_AND:
		operands := expr.GetBinaryOp()
		return containsIDFilter(operands.Lhs) ||
			containsIDFilter(operands.Rhs)
	case api.Expression_LOGICAL_OR:
		operands := expr.GetBinaryOp()
		return containsIDFilter(operands.Lhs) &&
			containsIDFilter(operands.Rhs)
	case api.Expression_EQ:
		operands := expr.GetBinaryOp()
		if operands.Lhs.GetType() != api.Expression_IDENTIFIER {
			return false
		}
		if operands.Lhs.GetIdentifier() != "id" {
			return false
		}
		return true
	}
	return false
}

func rewriteSyscallEventFilter(sef *api.SyscallEventFilter) {
	if sef.Id != nil {
		newExpr := expression.Equal(
			expression.Identifier("id"),
			expression.Value(sef.Id.Value))
		sef.FilterExpression = expression.LogicalAnd(
			newExpr, sef.FilterExpression)
		sef.Id = nil
	}

	if sef.Type == api.SyscallEventType_SYSCALL_EVENT_TYPE_ENTER {
		if sef.Arg0 != nil {
			newExpr := expression.Equal(
				expression.Identifier("arg0"),
				expression.Value(sef.Arg0.Value))
			sef.FilterExpression = expression.LogicalAnd(
				newExpr, sef.FilterExpression)
			sef.Arg0 = nil
		}

		if sef.Arg1 != nil {
			newExpr := expression.Equal(
				expression.Identifier("arg1"),
				expression.Value(sef.Arg1.Value))
			sef.FilterExpression = expression.LogicalAnd(
				newExpr, sef.FilterExpression)
			sef.Arg1 = nil
		}

		if sef.Arg2 != nil {
			newExpr := expression.Equal(
				expression.Identifier("arg2"),
				expression.Value(sef.Arg2.Value))
			sef.FilterExpression = expression.LogicalAnd(
				newExpr, sef.FilterExpression)
			sef.Arg2 = nil
		}

		if sef.Arg3 != nil {
			newExpr := expression.Equal(
				expression.Identifier("arg3"),
				expression.Value(sef.Arg3.Value))
			sef.FilterExpression = expression.LogicalAnd(
				newExpr, sef.FilterExpression)
			sef.Arg3 = nil
		}

		if sef.Arg4 != nil {
			newExpr := expression.Equal(
				expression.Identifier("arg4"),
				expression.Value(sef.Arg4.Value))
			sef.FilterExpression = expression.LogicalAnd(
				newExpr, sef.FilterExpression)
			sef.Arg4 = nil
		}

		if sef.Arg5 != nil {
			newExpr := expression.Equal(
				expression.Identifier("arg5"),
				expression.Value(sef.Arg5.Value))
			sef.FilterExpression = expression.LogicalAnd(
				newExpr, sef.FilterExpression)
			sef.Arg5 = nil
		}
	} else if sef.Type == api.SyscallEventType_SYSCALL_EVENT_TYPE_EXIT {
		if sef.Ret != nil {
			newExpr := expression.Equal(
				expression.Identifier("ret"),
				expression.Value(sef.Ret.Value))
			sef.FilterExpression = expression.LogicalAnd(
				newExpr, sef.FilterExpression)
			sef.Ret = nil
		}
	}
}

const (
	syscallNewEnterKprobeAddress string = "syscall_trace_enter_phase1"
	syscallOldEnterKprobeAddress string = "syscall_trace_enter"

	// These offsets index into the x86_64 version of struct pt_regs
	// in the kernel. This is a stable structure.
	syscallEnterKprobeFetchargs string = "id=+120(%di):s64 " + // orig_ax
		"arg0=+112(%di):u64 " + // di
		"arg1=+104(%di):u64 " + // si
		"arg2=+96(%di):u64 " + // dx
		"arg3=+56(%di):u64 " + // r10
		"arg4=+72(%di):u64 " + // r8
		"arg5=+64(%di):u64" // r9
)

func registerSyscallEvents(
	sensor *Sensor,
	subscr *subscription,
	events []*api.SyscallEventFilter,
) {
	var enterFilter, exitFilter *api.Expression

	for _, sef := range events {
		// Translate deprecated fields into an expression
		rewriteSyscallEventFilter(sef)

		if !containsIDFilter(sef.FilterExpression) {
			// No wildcard filters for now
			subscr.logStatus(
				code.Code_INVALID_ARGUMENT,
				"Wildcard syscall filter ignored")
			continue
		}

		switch sef.Type {
		case api.SyscallEventType_SYSCALL_EVENT_TYPE_ENTER:
			enterFilter = expression.LogicalOr(enterFilter,
				sef.FilterExpression)
		case api.SyscallEventType_SYSCALL_EVENT_TYPE_EXIT:
			exitFilter = expression.LogicalOr(exitFilter,
				sef.FilterExpression)
		default:
			subscr.logStatus(
				code.Code_INVALID_ARGUMENT,
				fmt.Sprintf("SyscallEventType %d is invalid", sef.Type))
			continue
		}
	}

	f := syscallFilter{
		sensor: sensor,
	}

	if enterFilter != nil {
		// Create the dummy syscall event. This event is needed to put
		// the kernel into a mode where it'll make the function calls
		// needed to make the kprobe we'll add fire. Add the tracepoint,
		// but make sure it never adds events into the ringbuffer by
		// using a filter that will never evaluate true. It also never
		// gets enabled, but just creating it is enough.
		//
		// For kernels older than 3.x, create this dummy event in all
		// event groups, because we cannot remove it when we don't need
		// it anymore due to bugs in CentOS 6.x kernels (2.6.32).
		var (
			err     error
			eventID uint64
		)
		eventName := "raw_syscalls/sys_enter"
		major, _, _ := sys.KernelVersion()
		if major < 3 {
			eventID, err = sensor.Monitor.RegisterTracepoint(
				eventName, f.decodeDummySysEnter,
				perf.WithEventGroup(subscr.eventGroupID),
				perf.WithFilter("id == 0x7fffffff"))
			if err != nil {
				eventName = "syscalls/sys_enter"
				eventID, err = sensor.Monitor.RegisterTracepoint(
					eventName, f.decodeDummySysEnter,
					perf.WithEventGroup(subscr.eventGroupID),
					perf.WithFilter("id == 0x7fffffff"))
			}
			if err != nil {
				subscr.logStatus(
					code.Code_UNKNOWN,
					fmt.Sprintf("Could not register dummy syscall event %s: %v", eventName, err))
			}
		} else if atomic.AddInt64(&sensor.dummySyscallEventCount, 1) == 1 {
			eventID, err = sensor.Monitor.RegisterTracepoint(
				eventName, f.decodeDummySysEnter,
				perf.WithEventGroup(0),
				perf.WithFilter("id == 0x7fffffff"))
			if err != nil {
				subscr.logStatus(
					code.Code_UNKNOWN,
					fmt.Sprintf("Could not register dummy syscall event %s: %v", eventName, err))
				atomic.AddInt64(&sensor.dummySyscallEventCount, -1)
			} else {
				sensor.dummySyscallEventID = eventID
			}
		}

		// There are two possible kprobes. Newer kernels (>= 4.1) have
		// refactored syscall entry code, so syscall_trace_enter_phase1
		// is the right one, but for older kernels syscall_trace_enter
		// is the right one. Both have the same signature, so the
		// fetchargs doesn't have to change. Try the new probe first,
		// because the old probe will also set in the newer kernels,
		// but it won't fire.
		kprobeSymbol := syscallNewEnterKprobeAddress
		eventID, err = sensor.RegisterKprobe(
			kprobeSymbol, false,
			syscallEnterKprobeFetchargs,
			f.decodeSyscallTraceEnter,
			perf.WithEventGroup(subscr.eventGroupID))
		if err != nil {
			kprobeSymbol = syscallOldEnterKprobeAddress
			eventID, err = sensor.RegisterKprobe(
				kprobeSymbol, false,
				syscallEnterKprobeFetchargs,
				f.decodeSyscallTraceEnter,
				perf.WithEventGroup(subscr.eventGroupID))
		}
		if err != nil {
			subscr.logStatus(
				code.Code_UNKNOWN,
				fmt.Sprintf("Could not register syscall enter kprobe %s: %v", kprobeSymbol, err))
		} else {
			es, err := subscr.addEventSink(eventID, enterFilter,
				syscallEnterEventTypes)
			if err != nil {
				subscr.logStatus(
					code.Code_UNKNOWN,
					fmt.Sprintf("Invalid filter expression for syscall enter filter: %v", err))
				sensor.Monitor.UnregisterEvent(eventID)
				if major >= 3 {
					eventID = sensor.dummySyscallEventID
					if atomic.AddInt64(&sensor.dummySyscallEventCount, -1) == 0 {
						sensor.Monitor.UnregisterEvent(sensor.dummySyscallEventID)
					}
				}
			} else if major >= 3 {
				es.unregister = func(*eventSink) {
					eventID := sensor.dummySyscallEventID
					if atomic.AddInt64(&sensor.dummySyscallEventCount, -1) == 0 {
						sensor.Monitor.UnregisterEvent(eventID)
					}
				}
			}
		}
	}

	if exitFilter != nil {
		eventName := "raw_syscalls/sys_exit"
		eventID, err := sensor.Monitor.RegisterTracepoint(eventName,
			f.decodeSysExit,
			perf.WithEventGroup(subscr.eventGroupID))
		if err != nil {
			eventName = "syscalls/sys_exit"
			eventID, err = sensor.Monitor.RegisterTracepoint(eventName,
				f.decodeSysExit,
				perf.WithEventGroup(subscr.eventGroupID))
		}
		if err != nil {
			subscr.logStatus(
				code.Code_UNKNOWN,
				fmt.Sprintf("Could not register tracepoint %s: %v", eventName, err))
		} else {
			_, err = subscr.addEventSink(eventID, exitFilter,
				syscallExitEventTypes)
			if err != nil {
				subscr.logStatus(
					code.Code_UNKNOWN,
					fmt.Sprintf("Invalid filter expression for syscall exit filter: %v", err))
				sensor.Monitor.UnregisterEvent(eventID)
			}
		}
	}
}
