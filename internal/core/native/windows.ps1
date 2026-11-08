$ErrorActionPreference='Stop'
Add-Type -AssemblyName System.Windows.Forms
Add-Type -AssemblyName System.Drawing
Add-Type -ReferencedAssemblies 'System.Windows.Forms.dll','System.Drawing.dll','System.dll','System.Core.dll' -TypeDefinition @'
using System;
using System.Runtime.InteropServices;
using System.Windows.Forms;
using System.Collections.Concurrent;
using System.Threading;
public static class PipeReader {
 public static ConcurrentQueue<string> Lines=new ConcurrentQueue<string>();
 public static volatile bool Ended;
 public static void Start(){var t=new Thread(()=>{try{string s;while((s=Console.In.ReadLine())!=null){if(s.Length>65536)break;Lines.Enqueue(s);}}finally{Ended=true;}});t.IsBackground=true;t.Start();}
 public static string Next(){string s;return Lines.TryDequeue(out s)?s:null;}
}
public class LocalControl : Form {
 [DllImport("user32.dll")] static extern bool RegisterHotKey(IntPtr h,int id,uint modifiers,uint key);
 [DllImport("user32.dll")] static extern bool UnregisterHotKey(IntPtr h,int id);
 delegate IntPtr Hook(int code,IntPtr w,IntPtr l);
 [DllImport("user32.dll")] static extern IntPtr SetWindowsHookEx(int id,Hook hook,IntPtr module,uint thread);
 [DllImport("user32.dll")] static extern bool UnhookWindowsHookEx(IntPtr hook);
 [DllImport("user32.dll")] static extern IntPtr CallNextHookEx(IntPtr h,int code,IntPtr w,IntPtr l);
 [DllImport("kernel32.dll")] static extern IntPtr GetModuleHandle(string name);
 Hook keyboard,mouse;IntPtr kh,mh;
 public event EventHandler PauseRequested;
 public bool BindTakeover(){
  keyboard=(code,w,l)=>{if(code>=0 && (w.ToInt32()==0x100||w.ToInt32()==0x104) && (Marshal.ReadInt32(l,8)&0x10)==0 && PauseRequested!=null)PauseRequested(this,EventArgs.Empty);return CallNextHookEx(kh,code,w,l);};
  mouse=(code,w,l)=>{if(code>=0 && (w.ToInt32()==0x200||w.ToInt32()==0x201||w.ToInt32()==0x204||w.ToInt32()==0x207||w.ToInt32()==0x20a) && (Marshal.ReadInt32(l,12)&1)==0 && PauseRequested!=null)PauseRequested(this,EventArgs.Empty);return CallNextHookEx(mh,code,w,l);};
  kh=SetWindowsHookEx(13,keyboard,GetModuleHandle(null),0);mh=SetWindowsHookEx(14,mouse,GetModuleHandle(null),0);return kh!=IntPtr.Zero && mh!=IntPtr.Zero;
 }
 [DllImport("wtsapi32.dll")] static extern bool WTSRegisterSessionNotification(IntPtr window,uint flags);
 [DllImport("wtsapi32.dll")] static extern bool WTSUnRegisterSessionNotification(IntPtr window);
 public bool BindSessionGuard(){return WTSRegisterSessionNotification(Handle,0);}
 public bool BindPause(){return RegisterHotKey(Handle,1,0x4003,0x50);}
 protected override void WndProc(ref Message m){if((m.Msg==0x312 || (m.Msg==0x2b1 && (m.WParam.ToInt32()==2 || m.WParam.ToInt32()==4 || m.WParam.ToInt32()==6 || m.WParam.ToInt32()==7))) && PauseRequested!=null)PauseRequested(this,EventArgs.Empty);base.WndProc(ref m);}
 protected override void OnFormClosed(FormClosedEventArgs e){UnregisterHotKey(Handle,1);WTSUnRegisterSessionNotification(Handle);if(kh!=IntPtr.Zero)UnhookWindowsHookEx(kh);if(mh!=IntPtr.Zero)UnhookWindowsHookEx(mh);base.OnFormClosed(e);}
}
'@
function Emit($event){[Console]::Out.WriteLine((@{event=$event}|ConvertTo-Json -Compress));[Console]::Out.Flush()}
$form=New-Object LocalControl
$form.Text='RelayDesk — kontrol lokal'
$form.Size=New-Object Drawing.Size(480,300)
$form.StartPosition='CenterScreen'
$status=New-Object Windows.Forms.Label
$status.Location=New-Object Drawing.Point(20,20);$status.Size=New-Object Drawing.Size(430,55)
$status.Text='Dijeda. Klik Resume untuk mengizinkan agent bekerja.'
$identity=New-Object Windows.Forms.Label
$identity.Location=New-Object Drawing.Point(20,80);$identity.Size=New-Object Drawing.Size(430,65)
$identity.Text='Gateway belum terhubung'
$hint=New-Object Windows.Forms.Label
$hint.Location=New-Object Drawing.Point(20,150);$hint.Size=New-Object Drawing.Size(430,40)
$form.Controls.AddRange(@($status,$identity,$hint))
$script:paused=$true
$pause={ if($script:paused){return};$script:paused=$true;$status.Text='Dijeda lokal. Job lama tidak dilanjutkan.';Emit 'pause' }
$form.add_PauseRequested($pause)
$i=0
foreach($name in @('Pause','Resume','Stop')){
 $button=New-Object Windows.Forms.Button
 $button.Text=$name;$button.Location=New-Object Drawing.Point((20+$i*140),200);$button.Size=New-Object Drawing.Size(130,35)
 if($name -eq 'Pause'){$button.add_Click($pause)}
 elseif($name -eq 'Resume'){$button.add_Click({$script:paused=$false;$status.Text='Siap; agent memerlukan lease baru.';Emit 'resume'})}
 else{$button.add_Click({Emit 'stop';$form.Close()})}
 $form.Controls.Add($button);$i++
}
$form.add_FormClosing({Emit 'stop'})
$timer=New-Object Windows.Forms.Timer
$timer.Interval=100
[PipeReader]::Start()
$timer.add_Tick({
 $line=[PipeReader]::Next()
 if($null -eq $line){if([PipeReader]::Ended){$form.Close()};return}
 try{
  $data=$line|ConvertFrom-Json
  if($data.op -eq 'status'){
   $s=$data.status
   $identity.Text="PC: $($s.device_name) $($s.device_id)`nAgent: $($s.agent_name) $($s.agent_id)"
   if(-not $script:paused){$status.Text="Status: $($s.state)"}
  }elseif($data.op -eq 'pause'){& $pause}elseif($data.op -eq 'stop'){$form.Close()}
 }catch{$form.Close()}
})
$form.add_Shown({
 if($form.BindPause()){$hint.Text='Shortcut global Pause: Ctrl+Alt+P.'}else{$hint.Text='Shortcut Ctrl+Alt+P bentrok. Gunakan tombol Pause.'}
 if($form.BindTakeover()){$hint.Text+=' Input fisik akan Pause.'}else{$hint.Text+=' Takeover otomatis gagal; gunakan Pause.'}
 if(-not $form.BindSessionGuard()){$hint.Text+=' Session guard gagal; Pause sebelum lock/disconnect.'}
 $timer.Start();Emit 'ready'
})
[Windows.Forms.Application]::Run($form)
$timer.Stop();$timer.Dispose();$form.Dispose()
