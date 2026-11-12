package core

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"os/exec"
	"strings"
	"time"
	"unicode/utf16"
)

const nonblockFileFlag = 0

func Capabilities() []string {
	return []string{"system_exec", "files", "desktop_windows_interactive", "browser_open"}
}
func ps(ctx context.Context, s string) *exec.Cmd {
	u := utf16.Encode([]rune(s))
	b := make([]byte, len(u)*2)
	for i, v := range u {
		b[2*i] = byte(v)
		b[2*i+1] = byte(v >> 8)
	}
	return exec.CommandContext(ctx, "powershell.exe", "-NoProfile", "-NonInteractive", "-STA", "-EncodedCommand", base64.StdEncoding.EncodeToString(b))
}
func shell(ctx context.Context, s string) *exec.Cmd { return ps(ctx, s) }
func configureProcess(c *exec.Cmd) {
	c.Cancel = func() error {
		if c.Process == nil {
			return nil
		}
		return exec.Command("taskkill.exe", "/PID", itoa(c.Process.Pid), "/T", "/F").Run()
	}
	c.WaitDelay = 2 * time.Second
}
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	b := []byte{}
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	return string(b)
}
func openURL(ctx context.Context, u string) (any, error) {
	result, e := run(ctx, ps(ctx, "$ErrorActionPreference='Stop'; Start-Process -FilePath '"+strings.ReplaceAll(u, "'", "''")+"'"))
	if e == nil && result["exit_code"] != 0 {
		return nil, errors.New(result["output"].(string))
	}
	return result, e
}
func desktop(ctx context.Context, action string, a map[string]any) (any, error) {
	b, _ := json.Marshal(map[string]any{"action": action, "args": a})
	s := winDesktop + "\n$p=[Text.Encoding]::UTF8.GetString([Convert]::FromBase64String('" + base64.StdEncoding.EncodeToString(b) + "')) | ConvertFrom-Json\n" + winDispatch
	c := ps(ctx, s)
	if action == "desktop_screenshot" {
		var out capped
		out.max = maxMessageBytes
		c.Stdout = &out
		var stderr capped
		stderr.max = 16384
		c.Stderr = &stderr
		configureProcess(c)
		if e := c.Run(); e != nil {
			return nil, errors.New(string(stderr.b))
		}
		if out.truncated {
			return nil, errors.New("screenshot too large")
		}
		var result map[string]any
		if e := json.Unmarshal(out.b, &result); e != nil {
			return nil, e
		}
		return result, nil
	}
	result, e := run(ctx, c)
	if e == nil && result["exit_code"] != 0 {
		return nil, errors.New(result["output"].(string))
	}
	return result, e
}

const winDesktop = `$ErrorActionPreference='Stop'
Add-Type -AssemblyName System.Windows.Forms
Add-Type -AssemblyName System.Drawing
Add-Type @'
using System;
using System.Runtime.InteropServices;
public class Desk {
 [DllImport("user32.dll")] public static extern bool SetCursorPos(int x,int y);
 [DllImport("user32.dll")] public static extern void mouse_event(uint f,uint x,uint y,int data,UIntPtr extra);
 [DllImport("user32.dll")] public static extern void keybd_event(byte key,byte scan,uint flags,UIntPtr extra);
 [StructLayout(LayoutKind.Sequential)] public struct INPUT { public uint type; public UNION u; }
 [StructLayout(LayoutKind.Explicit)] public struct UNION { [FieldOffset(0)] public KEY ki; [FieldOffset(0)] public MOUSE mi; }
 [StructLayout(LayoutKind.Sequential)] public struct KEY {public ushort vk,scan; public uint flags,time;public UIntPtr extra;}
 [StructLayout(LayoutKind.Sequential)] public struct MOUSE {public int x,y;public uint data,flags,time;public UIntPtr extra;}
 [DllImport("user32.dll",SetLastError=true)] public static extern uint SendInput(uint n,INPUT[] inputs,int size);
 [DllImport("user32.dll",SetLastError=true)] static extern IntPtr OpenInputDesktop(uint flags,bool inherit,uint access);
 [DllImport("user32.dll")] static extern bool CloseDesktop(IntPtr desktop);
 [DllImport("user32.dll",CharSet=CharSet.Unicode,SetLastError=true)] static extern bool GetUserObjectInformation(IntPtr obj,int index,System.Text.StringBuilder value,int length,out int needed);
 public static void CheckDesktop(){
  if(System.Diagnostics.Process.GetCurrentProcess().SessionId==0)throw new Exception("Interactive user session required");
  IntPtr d=OpenInputDesktop(0,false,1);if(d==IntPtr.Zero)throw new Exception("Input desktop unavailable: unlock locally or approve UAC locally");
  try{int n;var name=new System.Text.StringBuilder(256);if(!GetUserObjectInformation(d,2,name,512,out n)||!String.Equals(name.ToString(),"Default",StringComparison.OrdinalIgnoreCase))throw new Exception("Secure or locked desktop; user action required");}finally{CloseDesktop(d);}
 }
 public static void Type(string s){foreach(char ch in s){var a=new INPUT[2];a[0].type=1;a[0].u.ki.scan=ch;a[0].u.ki.flags=4;a[1]=a[0];a[1].u.ki.flags=6;if(SendInput(2,a,Marshal.SizeOf(typeof(INPUT)))!=2)throw new Exception("SendInput failed; check desktop/session privileges");}}
}
'@
`
const winDispatch = `[Desk]::CheckDesktop()
$a=$p.args
switch($p.action){
 'desktop_screenshot' {
 $r=[Windows.Forms.SystemInformation]::VirtualScreen
 $b=New-Object Drawing.Bitmap($r.Width,$r.Height)
 $g=[Drawing.Graphics]::FromImage($b)
 try {$g.CopyFromScreen($r.Left,$r.Top,0,0,$b.Size);$m=New-Object IO.MemoryStream;try{$b.Save($m,[Drawing.Imaging.ImageFormat]::Png);@{mimeType='image/png';base64=[Convert]::ToBase64String($m.ToArray());origin_x=$r.Left;origin_y=$r.Top;width=$r.Width;height=$r.Height}|ConvertTo-Json -Compress}finally{$m.Dispose()}}finally{$g.Dispose();$b.Dispose()}
 }
 'desktop_type' {[Desk]::Type([string]$a.text)}
 'desktop_move' {if(-not [Desk]::SetCursorPos([int]$a.x,[int]$a.y)){throw 'SetCursorPos failed'}}
 'desktop_click' {
 if(-not [Desk]::SetCursorPos([int]$a.x,[int]$a.y)){throw 'SetCursorPos failed'}
 $down=2;$up=4
 if($a.button -eq 'right'){$down=8;$up=16}elseif($a.button -eq 'middle'){$down=32;$up=64}elseif($a.button -and $a.button -ne 'left'){throw 'Unknown button'}
 $n=1;if($a.count){$n=[int]$a.count};if($n -lt 1 -or $n -gt 2){throw 'count must be 1 or 2'}
 for($i=0;$i -lt $n;$i++){[Desk]::mouse_event($down,0,0,0,[UIntPtr]::Zero);[Desk]::mouse_event($up,0,0,0,[UIntPtr]::Zero);Start-Sleep -Milliseconds 100}
 }
 'desktop_scroll' {if([Math]::Abs([int]$a.steps) -gt 100){throw 'steps must be -100..100'};[Desk]::mouse_event(2048,0,0,([int]$a.steps*120),[UIntPtr]::Zero)}
 'desktop_key' {
 $map=@{ctrl=17;shift=16;alt=18;win=91;enter=13;tab=9;esc=27;escape=27;backspace=8;delete=46;space=32;left=37;up=38;right=39;down=40;home=36;end=35;pageup=33;pagedown=34;insert=45}
 $keys=@();foreach($k in ([string]$a.key).ToLower().Split('+')){if($map.ContainsKey($k)){$keys+=[byte]$map[$k]}elseif($k -match '^f([1-9]|1[0-2])$'){$keys+=[byte](111+[int]$k.Substring(1))}elseif($k -match '^[a-z0-9]$'){$keys+=[byte][char]$k.ToUpper()}else{throw "Unsupported key: $k"}}
 try {foreach($k in $keys){[Desk]::keybd_event($k,0,0,[UIntPtr]::Zero)}}finally{[Array]::Reverse($keys);foreach($k in $keys){[Desk]::keybd_event($k,0,2,[UIntPtr]::Zero)}}
 }
 default {throw 'Unknown desktop action'}
}
`
