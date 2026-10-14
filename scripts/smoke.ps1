param([string]$BaseUrl = 'http://127.0.0.1:18080')
$ErrorActionPreference = 'Stop'
Set-Location -LiteralPath (Split-Path -Parent $PSScriptRoot)
$taskToken = (go run ./cmd/token -subject north-admin -tenant north-health).Trim()
if ($LASTEXITCODE -ne 0) { throw 'Token generation failed.' }
$taskHeaders = @{Authorization = "Bearer $taskToken"}
function Invoke-Healthcare([string]$Method, [string]$Path, $Body) {
    $taskRequestHeaders = $taskHeaders.Clone()
    $taskRequestHeaders['Idempotency-Key'] = [Guid]::NewGuid().ToString('N')
    $taskArguments = @{Method=$Method;Uri="$BaseUrl$Path";Headers=$taskRequestHeaders;ContentType='application/json'}
    if ($null -ne $Body) { $taskArguments['Body'] = ConvertTo-Json -InputObject $Body -Depth 8 }
    Invoke-RestMethod @taskArguments
}
function Wait-Workflow([string]$ID, [string]$Status) {
    for ($taskAttempt=0; $taskAttempt -lt 60; $taskAttempt++) {
        $taskWorkflow = Invoke-Healthcare GET "/api/v1/workflows/$ID" $null
        if ($taskWorkflow.status -eq $Status) { return $taskWorkflow }
        Start-Sleep -Milliseconds 200
    }
    throw "Workflow did not reach $Status."
}
$taskRunID = [Guid]::NewGuid().ToString('N')
$taskFacility = Invoke-Healthcare POST '/api/v1/facilities' @{name="Demo $taskRunID";capacity=2}
$taskStaff = Invoke-Healthcare POST '/api/v1/staff' @{facility_id=$taskFacility.id;name='Synthetic Nurse';qualifications=@('infusion')}
$taskChair = Invoke-Healthcare POST '/api/v1/care-resources' @{facility_id=$taskFacility.id;name='Chair 1';kind='chair'}
$taskPatient = Invoke-Healthcare POST '/api/v1/patients' @{facility_id=$taskFacility.id;external_id="synthetic-$taskRunID";name='Synthetic Patient';birth_date='1990-01-01'}
$taskOrder = Invoke-Healthcare POST '/api/v1/orders' @{patient_id=$taskPatient.id;medication='Synthetic medication';prescription='synthetic-prescription-reference'}
foreach ($taskOutcome in @(@{step='insurance';outcome='approved'},@{step='inventory';outcome='available'},@{step='clearance';outcome='approved'})) {
    $null = Invoke-Healthcare POST "/api/v1/workflows/$($taskOrder.workflow_id)/outcomes" $taskOutcome
}
$null = Wait-Workflow $taskOrder.workflow_id 'ready_to_schedule'
$taskStart = [DateTime]::UtcNow.AddHours(1)
$taskAppointment = Invoke-Healthcare POST '/api/v1/appointments' @{order_id=$taskOrder.id;nurse_id=$taskStaff.id;resource_id=$taskChair.id;starts_at=$taskStart.ToString('o');ends_at=$taskStart.AddHours(1).ToString('o')}
$null = Invoke-Healthcare POST "/api/v1/workflows/$($taskOrder.workflow_id)/outcomes" @{step='administration';outcome='completed'}
$null = Wait-Workflow $taskOrder.workflow_id 'awaiting_billing'
$null = Invoke-Healthcare POST "/api/v1/workflows/$($taskOrder.workflow_id)/outcomes" @{step='billing';outcome='submitted'}
$taskWorkflow = Wait-Workflow $taskOrder.workflow_id 'completed'
Write-Host "Smoke test passed. Workflow: $($taskWorkflow.id), appointment: $($taskAppointment.id)."
