package runner

import "testing"

func TestDefaultWindowsJobResourceLimitsAreFinite(t *testing.T) {
	limits := DefaultWindowsJobResourceLimits()
	if err := limits.Validate(); err != nil {
		t.Fatalf("default Windows Job Object limits are invalid: %v", err)
	}
	if limits.CPUPercent != 50 || limits.ProcessMemoryBytes != 2<<30 || limits.JobMemoryBytes != 2<<30 || limits.MaxActiveProcesses != 64 {
		t.Fatalf("unexpected default Windows Job Object limits: %+v", limits)
	}
	rate, err := limits.CPURateControlValue()
	if err != nil || rate != 5000 {
		t.Fatalf("Windows hard-cap CPU rate=%d err=%v, want 5000", rate, err)
	}
}

func TestWindowsJobResourceLimitsRejectUnboundedValues(t *testing.T) {
	base := DefaultWindowsJobResourceLimits()
	cases := []struct {
		name   string
		mutate func(*WindowsJobResourceLimits)
	}{
		{"zero CPU", func(limits *WindowsJobResourceLimits) { limits.CPUPercent = 0 }},
		{"CPU above host capacity", func(limits *WindowsJobResourceLimits) { limits.CPUPercent = 101 }},
		{"zero process memory", func(limits *WindowsJobResourceLimits) { limits.ProcessMemoryBytes = 0 }},
		{"zero job memory", func(limits *WindowsJobResourceLimits) { limits.JobMemoryBytes = 0 }},
		{"job memory below process memory", func(limits *WindowsJobResourceLimits) { limits.JobMemoryBytes = limits.ProcessMemoryBytes - 1 }},
		{"zero process count", func(limits *WindowsJobResourceLimits) { limits.MaxActiveProcesses = 0 }},
		{"excessive process count", func(limits *WindowsJobResourceLimits) { limits.MaxActiveProcesses = 257 }},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			limits := base
			testCase.mutate(&limits)
			if err := limits.Validate(); err == nil {
				t.Fatalf("invalid resource limits were accepted: %+v", limits)
			}
		})
	}
}
