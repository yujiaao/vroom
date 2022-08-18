package chrometrace

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"

	"github.com/getsentry/vroom/internal/aggregate"
	"github.com/getsentry/vroom/internal/android"
	"github.com/getsentry/vroom/internal/calltree"
	"github.com/getsentry/vroom/internal/errorutil"
	"github.com/getsentry/vroom/internal/snubautil"
)

type output struct {
	ActiveProfileIndex int           `json:"activeProfileIndex"`
	AndroidClock       android.Clock `json:"androidClock,omitempty"`
	DurationNS         uint64        `json:"durationNS"`
	Platform           string        `json:"platform"`
	ProfileID          string        `json:"profileID"`
	Profiles           []interface{} `json:"profiles"`
	ProjectID          uint64        `json:"projectID"`
	Shared             sharedData    `json:"shared"`
	TransactionName    string        `json:"transactionName"`
	Version            string        `json:"version"`
}

// SpeedscopeFromSnuba generates a profile using the Speedscope format from data in Snuba
func SpeedscopeFromSnuba(profile snubautil.Profile) ([]byte, error) {
	var p output
	switch profile.Platform {
	case "android":
		var androidProfile android.AndroidProfile
		err := json.Unmarshal([]byte(profile.Profile), &androidProfile)
		if err != nil {
			return nil, err
		}
		p, err = androidSpeedscopeTraceFromProfile(&androidProfile)
		if err != nil {
			return nil, err
		}
	case "cocoa":
		var iosProfile aggregate.IosProfile
		err := json.Unmarshal([]byte(profile.Profile), &iosProfile)
		if err != nil {
			return nil, err
		}
		p, err = iosSpeedscopeTraceFromProfile(&iosProfile)
		if err != nil {
			return nil, err
		}
	case "rust":
		var rustProfile aggregate.RustProfile
		err := json.Unmarshal([]byte(profile.Profile), &rustProfile)
		if err != nil {
			return nil, err
		}
		p, err = rustSpeedscopeTraceFromProfile(&rustProfile)
		if err != nil {
			return nil, err
		}
	case "python":
		var pythonProfile aggregate.PythonProfile
		err := json.Unmarshal([]byte(profile.Profile), &pythonProfile)
		if err != nil {
			return nil, err
		}
		p, err = pythonSpeedscopeTraceFromProfile(&pythonProfile)
		if err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("chrometrace: %w: %s is not a supported platform", errorutil.ErrDataIntegrity, profile.Platform)
	}
	p.DurationNS = profile.DurationNs
	p.Platform = profile.Platform
	p.ProfileID = profile.ProfileID
	p.ProjectID = profile.ProjectID
	p.TransactionName = profile.TransactionName
	p.Version = profile.Version()
	return json.Marshal(p)
}

func iosSpeedscopeTraceFromProfile(profile *aggregate.IosProfile) (output, error) {
	threadIDToProfile := make(map[uint64]*sampledProfile)
	addressToFrameIndex := make(map[string]int)
	threadIDToPreviousTimestampNS := make(map[uint64]uint64)
	frames := make([]frame, 0)
	// we need to find the frame index of the main function so we can remove the frames before it
	mainFunctionFrameIndex := -1
	mainThreadID := profile.MainThread()
	for _, sample := range profile.Samples {
		threadID := strconv.FormatUint(sample.ThreadID, 10)
		sampProfile, ok := threadIDToProfile[sample.ThreadID]
		queueMetadata, qmExists := profile.QueueMetadata[sample.QueueAddress]
		if !ok {
			threadMetadata, tmExists := profile.ThreadMetadata[threadID]
			threadName := threadMetadata.Name
			if threadName == "" && qmExists && (!queueMetadata.LabeledAsMainThread() || sample.ThreadID != mainThreadID) {
				threadName = queueMetadata.Label
			} else {
				threadName = threadID
			}
			sampProfile = &sampledProfile{
				Name:         threadName,
				Queues:       make(map[string]queue),
				StartValue:   sample.RelativeTimestampNS,
				ThreadID:     sample.ThreadID,
				IsMainThread: sample.ThreadID == mainThreadID,
				Type:         profileTypeSampled,
				Unit:         valueUnitNanoseconds,
			}
			if qmExists {
				sampProfile.Queues[queueMetadata.Label] = queue{Label: queueMetadata.Label, StartNS: sample.RelativeTimestampNS, EndNS: sample.RelativeTimestampNS}
			}
			if tmExists {
				sampProfile.Priority = threadMetadata.Priority
			}
			threadIDToProfile[sample.ThreadID] = sampProfile
		} else {
			if qmExists {
				q, qExists := sampProfile.Queues[queueMetadata.Label]
				if !qExists {
					sampProfile.Queues[queueMetadata.Label] = queue{Label: queueMetadata.Label, StartNS: sample.RelativeTimestampNS, EndNS: sample.RelativeTimestampNS}
				} else {
					q.EndNS = sample.RelativeTimestampNS
					sampProfile.Queues[queueMetadata.Label] = q
				}
			}
			sampProfile.Weights = append(sampProfile.Weights, sample.RelativeTimestampNS-threadIDToPreviousTimestampNS[sample.ThreadID])
		}

		sampProfile.EndValue = sample.RelativeTimestampNS
		threadIDToPreviousTimestampNS[sample.ThreadID] = sample.RelativeTimestampNS

		samp := make([]int, 0, len(sample.Frames))
		for i := len(sample.Frames) - 1; i >= 0; i-- {
			fr := sample.Frames[i]
			frameIndex, ok := addressToFrameIndex[fr.InstructionAddr]
			if !ok {
				frameIndex = len(frames)
				symbolName := fr.Function
				if symbolName == "" {
					symbolName = fmt.Sprintf("unknown (%s)", fr.InstructionAddr)
				} else if mainFunctionFrameIndex == -1 {
					if isMainFrame, i := fr.IsMain(); isMainFrame {
						mainFunctionFrameIndex = frameIndex + i
					}
				}
				addressToFrameIndex[fr.InstructionAddr] = frameIndex
				frames = append(frames, frame{
					File:          fr.Filename,
					Image:         calltree.ImageBaseName(fr.Package),
					IsApplication: aggregate.IsIOSApplicationImage(fr.Package),
					Line:          fr.LineNo,
					Name:          symbolName,
				})
			}
			samp = append(samp, frameIndex)
		}
		sampProfile.Samples = append(sampProfile.Samples, samp)
	} // end loop sampledProfiles
	var mainThreadProfileIndex int
	allProfiles := make([]interface{}, 0)
	for _, prof := range threadIDToProfile {
		if prof.IsMainThread {
			mainThreadProfileIndex = len(allProfiles)
			// Remove all frames before main is called on the main thread
			if mainFunctionFrameIndex != -1 {
				for i, sample := range prof.Samples {
					for j, frameIndex := range sample {
						if frameIndex == mainFunctionFrameIndex {
							prof.Samples[i] = prof.Samples[i][j:]
							break
						}
					}
				}
			}
		}
		prof.Weights = append(prof.Weights, 0)
		allProfiles = append(allProfiles, prof)
	}
	return output{
		ActiveProfileIndex: mainThreadProfileIndex,
		Profiles:           allProfiles,
		Shared:             sharedData{Frames: frames},
	}, nil
}

func androidSpeedscopeTraceFromProfile(profile *android.AndroidProfile) (output, error) {
	frames := make([]frame, 0)
	methodIDToFrameIndex := make(map[uint64][]int)
	for _, method := range profile.Methods {
		if len(method.InlineFrames) > 0 {
			for _, m := range method.InlineFrames {
				methodIDToFrameIndex[method.ID] = append(methodIDToFrameIndex[method.ID], len(frames))
				frames = append(frames, frame{
					File:          m.SourceFile,
					Image:         m.ClassName,
					Inline:        true,
					IsApplication: !aggregate.IsAndroidSystemPackage(m.ClassName),
					Line:          m.SourceLine,
					Name:          m.Name,
				})

			}
		} else {
			packageName, _, err := android.ExtractPackageNameAndSimpleMethodNameFromAndroidMethod(&method)
			if err != nil {
				return output{}, err
			}
			fullMethodName, err := android.FullMethodNameFromAndroidMethod(&method)
			if err != nil {
				return output{}, err
			}
			methodIDToFrameIndex[method.ID] = append(methodIDToFrameIndex[method.ID], len(frames))
			frames = append(frames, frame{
				Name:          fullMethodName,
				File:          method.SourceFile,
				Line:          method.SourceLine,
				IsApplication: !aggregate.IsAndroidSystemPackage(fullMethodName),
				Image:         packageName,
			})
		}
	}

	emitEvent := func(profile *eventedProfile, et eventType, methodID, ts uint64) error {
		frameIndexes, ok := methodIDToFrameIndex[methodID]
		if !ok {
			// sometimes it might happen that a method is listed in events but an entry definition
			// is not correctly defined in the methods entry. We don't wan't to fail the whole chrometrace
			// for this so we create a method on the fly
			frameIndexes = []int{len(frames)}
			methodIDToFrameIndex[methodID] = append(methodIDToFrameIndex[methodID], frameIndexes[0])
			frames = append(frames, frame{
				Name:          fmt.Sprintf("unknown (id %d)", methodID),
				File:          "unknown",
				Line:          0,
				IsApplication: false,
				Image:         "unknown",
			})
		}
		for _, fi := range frameIndexes {
			profile.Events = append(profile.Events, event{
				Type:  et,
				Frame: fi,
				At:    ts,
			})
		}
		return nil
	}

	threadIDToProfile := make(map[uint64]*eventedProfile)
	methodStacks := make(map[uint64][]uint64) // map of thread ID -> stack of method IDs
	buildTimestamp := profile.TimestampGetter()

	for _, event := range profile.Events {
		ts := buildTimestamp(event.Time)
		prof, ok := threadIDToProfile[event.ThreadID]
		if !ok {
			threadID := event.ThreadID
			prof = &eventedProfile{
				StartValue: ts,
				ThreadID:   threadID,
				Type:       profileTypeEvented,
				Unit:       valueUnitNanoseconds,
			}
			threadIDToProfile[threadID] = prof
		}
		prof.EndValue = ts

		switch event.Action {
		case "Enter":
			methodStacks[event.ThreadID] = append(methodStacks[event.ThreadID], event.MethodID)
			if err := emitEvent(prof, eventTypeOpenFrame, event.MethodID, ts); err != nil {
				return output{}, err
			}
		case "Exit", "Unwind":
			stack := methodStacks[event.ThreadID]
			if len(stack) == 0 {
				return output{}, fmt.Errorf("chrometrace: %w: ending event %v but stack for thread %v is empty", errorutil.ErrDataIntegrity, event, event.ThreadID)
			}
			i := len(stack) - 1
			// Iterate from top -> bottom of stack, looking for the method we're attempting to end.
			// Typically, this method should be on the top of the stack, but we may also be trying to
			// end a method before explicitly ending the child methods that are on top of that method
			// in the stack. In this scenario, we will synthesize end events for all methods that have
			// not been explicitly ended, matching the behavior of the Chrome trace viewer. Speedscope
			// handles this scenario a different way by doing nothing and leaving these methods with
			// indefinite durations.
			for ; i >= 0; i-- {
				methodID := stack[i]
				if err := emitEvent(prof, eventTypeCloseFrame, methodID, ts); err != nil {
					return output{}, err
				}
				if methodID == event.MethodID {
					break
				}
			}
			if stack[i] != event.MethodID {
				return output{}, fmt.Errorf("chrometrace: %w: ending event %v but stack for thread %v does not contain that record", errorutil.ErrDataIntegrity, event, event.ThreadID)
			}
			// Pop the elements that we emitted end events for off the stack
			methodStacks[event.ThreadID] = methodStacks[event.ThreadID][:i]

		default:
			return output{}, fmt.Errorf("chrometrace: %w: invalid method action: %v", errorutil.ErrDataIntegrity, event.Action)
		} // end switch
	} // end loop events

	// Close any remaining open frames.
	for threadID, stack := range methodStacks {
		prof := threadIDToProfile[threadID]
		for i := len(stack) - 1; i >= 0; i-- {
			if err := emitEvent(prof, eventTypeCloseFrame, stack[i], prof.EndValue); err != nil {
				return output{}, err
			}
		}
	}

	allProfiles := make([]interface{}, 0)
	var mainThreadProfileIndex int
	for _, thread := range profile.Threads {
		prof, ok := threadIDToProfile[thread.ID]
		if !ok {
			continue
		}
		if thread.Name == "main" {
			mainThreadProfileIndex = len(allProfiles)
		}
		prof.Name = thread.Name
		allProfiles = append(allProfiles, prof)
	}
	return output{
		ActiveProfileIndex: mainThreadProfileIndex,
		AndroidClock:       profile.Clock,
		Profiles:           allProfiles,
		Shared:             sharedData{Frames: frames},
	}, nil
}

func pythonSpeedscopeTraceFromProfile(profile *aggregate.PythonProfile) (output, error) {
	threadIDToProfile := make(map[uint64]*sampledProfile)
	threadIDToPreviousTimestampNS := make(map[uint64]uint64)

	sort.Slice(profile.Samples, func(i, j int) bool {
		return profile.Samples[i].RelativeTimestampNS <= profile.Samples[j].RelativeTimestampNS
	})
	for _, sample := range profile.Samples {
		sampProfile, ok := threadIDToProfile[sample.ThreadID]
		if !ok {
			sampProfile = &sampledProfile{
				Name:         strconv.FormatUint(sample.ThreadID, 10),
				Queues:       nil,
				StartValue:   sample.RelativeTimestampNS,
				ThreadID:     sample.ThreadID,
				IsMainThread: false,
				Type:         profileTypeSampled,
				Unit:         valueUnitNanoseconds,
			}
			threadIDToProfile[sample.ThreadID] = sampProfile
		} else {
			sampProfile.Weights = append(sampProfile.Weights, sample.RelativeTimestampNS-threadIDToPreviousTimestampNS[sample.ThreadID])
		}

		samp := make([]int, 0, len(sample.Frames))
		for i := len(sample.Frames) - 1; i >= 0; i-- {
			samp = append(samp, sample.Frames[i])
		}

		sampProfile.Samples = append(sampProfile.Samples, samp)
		sampProfile.EndValue = sample.RelativeTimestampNS
		threadIDToPreviousTimestampNS[sample.ThreadID] = sample.RelativeTimestampNS
	}

	frames := make([]frame, 0, len(profile.Frames))
	for _, pythonFrame := range profile.Frames {
		frames = append(frames, frame{
			File: pythonFrame.File,
			Name: pythonFrame.Name,
			Line: pythonFrame.Line,
		})
	}

	mainThreadProfileIndex := 0
	var mainThreadID uint64 = 0

	allProfiles := make([]interface{}, 0)
	for threadID, prof := range threadIDToProfile {
		if threadID < mainThreadID {
			mainThreadID = threadID
		}
		prof.Weights = append(prof.Weights, 0)
		allProfiles = append(allProfiles, prof)
	}

	return output{
		ActiveProfileIndex: mainThreadProfileIndex,
		Profiles:           allProfiles,
		Shared:             sharedData{Frames: frames},
	}, nil
}

func rustSpeedscopeTraceFromProfile(profile *aggregate.RustProfile) (output, error) {
	threadIDToProfile := make(map[uint64]*sampledProfile)
	addressToFrameIndex := make(map[string]int)
	threadIDToPreviousTimestampNS := make(map[uint64]uint64)
	frames := make([]frame, 0)
	// we need to find the frame index of the main function so we can remove the frames before it
	mainFunctionFrameIndex := -1
	mainThreadID := profile.MainThread()
	// sorting here is necessary because the timing info for each sample is given by
	// a Rust SystemTime type, which is measurement of the system clock and is not monotonic
	//
	// see: https://doc.rust-lang.org/std/time/struct.SystemTime.html
	sort.Slice(profile.Samples, func(i, j int) bool {
		return profile.Samples[i].RelativeTimestampNS <= profile.Samples[j].RelativeTimestampNS
	})
	for _, sample := range profile.Samples {
		sampProfile, ok := threadIDToProfile[sample.ThreadID]
		if !ok {
			threadName := sample.ThreadName
			// the rust profiler automatically use thread_id as a thread_name
			// when the thread_name is not available.
			// So if thread_name == mainThreadID we now it's the main thread
			// and we can replace it with `main`
			if threadName == strconv.FormatUint(mainThreadID, 10) {
				threadName = "main"
			}
			sampProfile = &sampledProfile{
				Name:         threadName,
				Queues:       nil,
				StartValue:   sample.RelativeTimestampNS,
				ThreadID:     sample.ThreadID,
				IsMainThread: sample.ThreadID == mainThreadID,
				Type:         profileTypeSampled,
				Unit:         valueUnitNanoseconds,
			}
			threadIDToProfile[sample.ThreadID] = sampProfile
		} else {
			sampProfile.Weights = append(sampProfile.Weights, sample.RelativeTimestampNS-threadIDToPreviousTimestampNS[sample.ThreadID])
		}

		sampProfile.EndValue = sample.RelativeTimestampNS
		threadIDToPreviousTimestampNS[sample.ThreadID] = sample.RelativeTimestampNS
		samp := make([]int, 0, len(sample.Frames))
		for i := len(sample.Frames) - 1; i >= 0; i-- {
			fr := sample.Frames[i]
			var address string
			if fr.SymAddr != "" {
				address = fr.SymAddr
			} else {
				address = fr.InstructionAddr
			}
			frameIndex, ok := addressToFrameIndex[address]
			if !ok {
				frameIndex = len(frames)
				symbolName := fr.Function
				if symbolName == "" {
					symbolName = fmt.Sprintf("unknown (%s)", fr.InstructionAddr)
				} else if mainFunctionFrameIndex == -1 {
					if isMainFrame := fr.IsMain(); isMainFrame {
						mainFunctionFrameIndex = frameIndex
					}
				}
				addressToFrameIndex[address] = frameIndex
				frames = append(frames, frame{
					File:          fr.Filename,
					Image:         calltree.ImageBaseName(fr.Package),
					Inline:        fr.Status == "symbolicated" && fr.SymAddr == "",
					IsApplication: aggregate.IsRustApplicationImage(fr.Package),
					Line:          fr.LineNo,
					Name:          symbolName,
				})
			}
			samp = append(samp, frameIndex)
		}
		sampProfile.Samples = append(sampProfile.Samples, samp)
	} // end loop sampledProfiles

	var mainThreadProfileIndex int
	allProfiles := make([]interface{}, 0)
	for _, prof := range threadIDToProfile {
		if prof.IsMainThread {
			mainThreadProfileIndex = len(allProfiles)
			// Remove all frames before main is called on the main thread
			if mainFunctionFrameIndex != -1 {
				for i, sample := range prof.Samples {
					for j, frameIndex := range sample {
						if frameIndex == mainFunctionFrameIndex {
							prof.Samples[i] = prof.Samples[i][j:]
							break
						}
					}
				}
			}
		}
		prof.Weights = append(prof.Weights, 0)
		allProfiles = append(allProfiles, prof)
	}

	return output{
		ActiveProfileIndex: mainThreadProfileIndex,
		Profiles:           allProfiles,
		Shared:             sharedData{Frames: frames},
	}, nil
}
