$ErrorActionPreference='Stop'
$output='C:\Results'
try {
    'started' | Set-Content (Join-Path $output 'started.txt')
    if(Get-Command node.exe,dotnet.exe -ErrorAction SilentlyContinue){throw 'Sandbox unexpectedly contains developer runtimes'}
    Copy-Item -LiteralPath 'C:\Release' -Destination 'C:\App' -Recurse
    'copied' | Set-Content (Join-Path $output 'copied.txt')
    & C:\App\Install-Prerequisites.ps1 *> (Join-Path $output 'prerequisites.log')
    'installed' | Set-Content (Join-Path $output 'installed.txt')
    Add-Type -AssemblyName UIAutomationClient,UIAutomationTypes,System.Drawing,System.Windows.Forms
    $process=Start-Process -FilePath 'C:\App\RouterWorkbench.exe' -WorkingDirectory 'C:\App' -WindowStyle Hidden -PassThru
    $found=$false
    for($attempt=0;$attempt -lt 120;$attempt++) {
        if($process.HasExited){throw "Product exited early: $($process.ExitCode)"}
        $owner=New-Object System.Windows.Automation.PropertyCondition([System.Windows.Automation.AutomationElement]::ProcessIdProperty,$process.Id)
        $title=New-Object System.Windows.Automation.PropertyCondition([System.Windows.Automation.AutomationElement]::NameProperty,'远程维护工作台')
        $condition=New-Object System.Windows.Automation.AndCondition($owner,$title)
        $window=[System.Windows.Automation.AutomationElement]::RootElement.FindFirst([System.Windows.Automation.TreeScope]::Children,$condition)
        if($window){
            $elements=$window.FindAll([System.Windows.Automation.TreeScope]::Descendants,[System.Windows.Automation.Condition]::TrueCondition)
            $names=@($elements | ForEach-Object {$_.Current.Name})
            if($names -contains '远程维护' -and $names -contains '开启维护'){$found=$true;break}
        }
        Start-Sleep -Milliseconds 500
    }
    $names | Set-Content (Join-Path $output 'accessible-ui.txt') -Encoding UTF8
    $bounds=[System.Windows.Forms.Screen]::PrimaryScreen.Bounds
    $bitmap=New-Object Drawing.Bitmap($bounds.Width,$bounds.Height)
    $graphics=[Drawing.Graphics]::FromImage($bitmap)
    $graphics.CopyFromScreen($bounds.Location,[Drawing.Point]::Empty,$bounds.Size)
    $bitmap.Save((Join-Path $output 'sandbox.png'));$graphics.Dispose();$bitmap.Dispose()
    if(!$found){throw 'React UI was not found in production WebView2 accessibility tree'}
    $web=Get-CimInstance Win32_Process -Filter "Name='msedgewebview2.exe'" | Where-Object {$_.ExecutablePath -eq 'C:\App\WebView2Runtime\msedgewebview2.exe'}
    if(!$web){throw 'Product did not load bundled WebView2 runtime'}
    $process.Refresh()
    if(!($process.Modules | Where-Object {$_.ModuleName -eq 'Microsoft.UI.Xaml.dll' -and $_.FileName -eq 'C:\App\Microsoft.UI.Xaml.dll'})){throw 'WinUI was not loaded from release directory'}
    if(!$process.CloseMainWindow() -or !$process.WaitForExit(15000) -or $process.ExitCode -ne 0){throw 'Product failed graceful exit'}
    @('PASS isolated Windows Sandbox','PASS networking disabled by WSB configuration','PASS Node and dotnet commands absent','PASS offline prerequisites','PASS production React accessibility tree','PASS bundled WebView2 and WinUI runtime','PASS normal WM_CLOSE exit=0') | Set-Content (Join-Path $output 'result.txt') -Encoding UTF8
} catch {
    $_ | Out-String | Set-Content (Join-Path $output 'failure.txt') -Encoding UTF8
    $diagnostic=Join-Path $env:LOCALAPPDATA 'RouterWorkbench\last-error.log'
    if(Test-Path $diagnostic){Copy-Item $diagnostic $output}
} finally {
    shutdown.exe /s /t 2
}
