<#
.SYNOPSIS
Measures Querra's startup time, memory and grid scrolling on Windows (WebView2).

.DESCRIPTION
Drives PerfService's autorun modes (QUERRA_PERF_AUTORUN, see
internal/app/perfservice.go) and prints a JSON summary, mirroring
dev/perf/measure_macos.py. Build first with `wails3 task package`.

.EXAMPLE
pwsh dev/perf/measure_windows.ps1 -Exe bin/querra.exe
#>
param(
    [string]$Exe = 'bin/querra.exe',
    [int]$Runs = 5,
    [int]$IdleSeconds = 10,
    [int]$SettleSeconds = 20
)

$ErrorActionPreference = 'Stop'
$Exe = (Resolve-Path $Exe).Path
$work = New-Item -ItemType Directory -Force -Path (Join-Path ([IO.Path]::GetTempPath()) "querra-perf-$PID")
# Starts every report line; matches perf.ReportPrefix in internal/perf/report.go.
$Prefix = 'PERF '

function Get-PerfEvents([string]$Report) {
    if (-not (Test-Path $Report)) { return @() }
    Get-Content $Report | Where-Object { $_.StartsWith($Prefix) } |
        ForEach-Object { $_.Substring($Prefix.Length) | ConvertFrom-Json }
}

function Wait-PerfEvent([string]$Report, [string]$Name, [int]$TimeoutSeconds) {
    $deadline = (Get-Date).AddSeconds($TimeoutSeconds)
    while ((Get-Date) -lt $deadline) {
        $found = Get-PerfEvents $Report | Where-Object event -EQ $Name | Select-Object -First 1
        if ($found) { return $found }
        Start-Sleep -Milliseconds 50
    }
    throw "no '$Name' event within ${TimeoutSeconds}s"
}

function Start-Perf([string]$Mode, [string]$Report, [hashtable]$Extra = @{}) {
    $vars = @{ QUERRA_PERF_AUTORUN = $Mode; QUERRA_PERF_REPORT = $Report } + $Extra
    foreach ($k in $vars.Keys) { Set-Item "env:$k" $vars[$k] }
    try { Start-Process -FilePath $Exe -PassThru }
    finally { foreach ($k in $vars.Keys) { Remove-Item "env:$k" } }
}

# WebView2 runs its msedgewebview2.exe processes as descendants of the app.
function Get-ProcessTree([int]$RootId) {
    $all = Get-CimInstance Win32_Process | Select-Object ProcessId, ParentProcessId
    $ids = [System.Collections.Generic.List[int]]::new()
    $ids.Add($RootId)
    for ($i = 0; $i -lt $ids.Count; $i++) {
        foreach ($p in $all) {
            if ($p.ParentProcessId -eq $ids[$i] -and -not $ids.Contains([int]$p.ProcessId)) {
                $ids.Add([int]$p.ProcessId)
            }
        }
    }
    $ids
}

# Private working set: what Task Manager shows as Memory.
function Get-TreeMemory([int]$RootId) {
    $ids = Get-ProcessTree $RootId
    $procs = [ordered]@{}
    Get-CimInstance Win32_PerfFormattedData_PerfProc_Process |
        Where-Object { $ids -contains [int]$_.IDProcess } |
        ForEach-Object { $procs["$($_.Name)[$($_.IDProcess)]"] = [math]::Round($_.WorkingSetPrivate / 1MB, 1) }
    [ordered]@{
        totalMB   = [math]::Round(($procs.Values | Measure-Object -Sum).Sum, 1)
        processes = $procs
    }
}

function Stop-Tree([int]$RootId) {
    Get-ProcessTree $RootId | ForEach-Object { Stop-Process -Id $_ -Force -ErrorAction SilentlyContinue }
}

function Measure-Startup {
    $samples = foreach ($i in 1..$Runs) {
        $report = Join-Path $work "startup-$i.jsonl"
        $launched = [DateTimeOffset]::UtcNow.ToUnixTimeMilliseconds()
        $proc = Start-Perf 'startup' $report
        $ready = Wait-PerfEvent $report 'ready' 60
        $null = $proc.WaitForExit(30000)
        [ordered]@{ wallMs = $ready.unixMs - $launched; sinceGoStartMs = $ready.sinceProcessStartMs }
        Start-Sleep -Seconds 1
    }
    $wall = $samples | ForEach-Object { $_.wallMs } | Sort-Object
    [ordered]@{
        runs     = @($samples)
        firstMs  = $samples[0].wallMs
        medianMs = $wall[[math]::Floor($wall.Count / 2)]
        maxMs    = $wall[-1]
    }
}

function Measure-Idle {
    $report = Join-Path $work 'idle.jsonl'
    $proc = Start-Perf 'idle' $report
    try {
        $null = Wait-PerfEvent $report 'ready' 60
        Start-Sleep -Seconds $IdleSeconds
        $memory = Get-TreeMemory $proc.Id
        [ordered]@{ afterSeconds = $IdleSeconds; totalMB = $memory.totalMB; processes = $memory.processes }
    }
    finally { Stop-Tree $proc.Id }
}

function Measure-Grid {
    $report = Join-Path $work 'grid.jsonl'
    $proc = Start-Perf 'grid' $report @{ QUERRA_PERF_LINGER_MS = "$(($SettleSeconds + 10) * 1000)" }
    try {
        $null = Wait-PerfEvent $report 'grid-loaded' 120
        $loaded = Get-TreeMemory $proc.Id
        $grid = Wait-PerfEvent $report 'grid' 300
        $afterBench = Get-TreeMemory $proc.Id
        Start-Sleep -Seconds $SettleSeconds
        $settled = Get-TreeMemory $proc.Id
        $null = $proc.WaitForExit(60000)
    }
    finally { Stop-Tree $proc.Id }
    [ordered]@{
        phases                = $grid.phases
        fetchRows             = $grid.fetch
        memoryDatasetLoaded   = $loaded
        memoryAfterBenchmark  = $afterBench
        memoryAfterSettle     = $settled
        goMemoryAtFinish      = (Get-PerfEvents $report | Where-Object event -EQ 'finish' | Select-Object -First 1).mem
    }
}

function Get-Sizes {
    $sizes = [ordered]@{ exeMB = [math]::Round((Get-Item $Exe).Length / 1MB, 1) }
    Get-ChildItem -Recurse -Path bin, build/windows/nsis -Filter '*installer*.exe' -ErrorAction SilentlyContinue |
        ForEach-Object { $sizes["installerMB:$($_.Name)"] = [math]::Round($_.Length / 1MB, 1) }
    $sizes
}

[ordered]@{
    exe        = $Exe
    sizes      = Get-Sizes
    startup    = Measure-Startup
    idleMemory = Measure-Idle
    grid       = Measure-Grid
} | ConvertTo-Json -Depth 8
