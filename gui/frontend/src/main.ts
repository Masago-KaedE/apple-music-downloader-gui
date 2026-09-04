import "./style.css";

type Settings = {
  projectRoot: string; distribution: string; storefront: string;
  alacSaveFolder: string; atmosSaveFolder: string; aacSaveFolder: string;
  embedCover: boolean; coverSize: string; coverFormat: string;
  embedLyrics: boolean; saveLyricsFile: boolean; lyricsFormat: string;
  aacType: string; alacMax: number; atmosMax: number;
};
type WrapperStatus = { ready: boolean; ownedByGUI: boolean; running: boolean; needs2FA: boolean; distribution: string; ports: {port:number; listening:boolean}[]; message:string };
type DownloadState = { running:boolean; canceled:boolean; phase:string; queue:number; queueSize:number; track:number; tracks:number; completed:number; warnings:number; errors:number; results:{path:string;artist:string;album:string;song:string;status:string}[]; message:string };
type Snapshot = { settings:Settings; project:{valid:boolean;description:string}; wrapper:WrapperStatus; download:DownloadState; distros:string[] };

const $ = <T extends HTMLElement = HTMLElement>(id: string) => document.getElementById(id) as T;
let snapshot: Snapshot | null = null;
let selectedQuality = "alac";
let logCount = 0;

async function invoke<T>(name: string, ...args: unknown[]): Promise<T> {
  const method = window.go?.main?.App?.[name];
  if (!method) throw new Error(`后端方法不可用：${name}`);
  return method(...args) as Promise<T>;
}

function showToast(message: string, error = false) {
  const toast = $("toast");
  toast.textContent = message;
  toast.className = `toast show ${error ? "error" : ""}`;
  window.setTimeout(() => toast.className = "toast", 3200);
}

function setPage(page: string) {
  document.querySelectorAll(".nav").forEach(el => el.classList.toggle("active", (el as HTMLElement).dataset.page === page));
  document.querySelectorAll(".page").forEach(el => el.classList.toggle("active", el.id === `page-${page}`));
  $("page-title").textContent = page === "download" ? "下载控制台" : page === "wrapper" ? "Wrapper 管理" : "安全设置";
}

function renderSnapshot(value: Snapshot) {
  snapshot = value;
  renderProject(value.project);
  renderWrapper(value.wrapper, value.distros);
  renderDownload(value.download);
  fillSettings(value.settings);
}

function renderProject(project: Snapshot["project"]) {
  $("project-dot").classList.toggle("ok", project.valid);
  $("project-label").textContent = project.valid ? "组件完整" : project.description;
}

function renderWrapper(status: WrapperStatus, distros = snapshot?.distros ?? []) {
  const chip = $("wrapper-chip");
  chip.classList.toggle("ready", status.ready);
  chip.querySelector("span:last-child")!.textContent = status.ready ? "Wrapper 已就绪" : "Wrapper 未就绪";
  $("wrapper-message").textContent = status.message;
  const ports = $("ports"); ports.replaceChildren();
  status.ports.forEach(item => {
    const node = document.createElement("div"); node.className = `port ${item.listening ? "ready" : ""}`;
    const label = document.createElement("strong"); label.textContent = String(item.port);
    const detail = document.createElement("small"); detail.textContent = item.listening ? "正在监听" : "不可用";
    node.append(label, detail); ports.append(node);
  });
  const select = $("distro-select") as HTMLSelectElement;
  const values = distros.length ? distros : [status.distribution || snapshot?.settings.distribution || "Ubuntu"];
  const current = select.value || snapshot?.settings.distribution || "Ubuntu";
  select.replaceChildren(...values.map(value => new Option(value, value, false, value === current)));
  $("twofactor").classList.toggle("hidden", !status.needs2FA);
  ($("stop-wrapper") as HTMLButtonElement).disabled = !status.ownedByGUI;
}

const phaseNames: Record<string,string> = { idle:"空闲", queued:"排队", starting:"启动", preparing:"准备", metadata:"歌词与元数据", downloading:"下载与解密", tagging:"写入标签", existing:"本地已存在", finished:"完成" };
function renderDownload(state: DownloadState) {
  $("download-message").textContent = state.message || "等待任务";
  $("phase-label").textContent = phaseNames[state.phase] || state.phase || "空闲";
  $("queue-metric").textContent = `${state.queue} / ${state.queueSize}`;
  $("track-metric").textContent = `${state.track} / ${state.tracks}`;
  $("complete-metric").textContent = String(state.completed);
  $("warning-metric").textContent = String(state.warnings);
  $("error-metric").textContent = String(state.errors);
  const queueProgress = state.queueSize ? ((Math.max(0, state.queue - 1) + (state.tracks ? state.track / state.tracks : 0)) / state.queueSize) * 100 : 0;
  ($("progress-fill") as HTMLElement).style.width = `${Math.min(100, Math.max(0, queueProgress))}%`;
  ($("start-download") as HTMLButtonElement).disabled = state.running;
  ($("cancel-download") as HTMLButtonElement).disabled = !state.running;
  renderResults(state.results);
}

function renderResults(results: DownloadState["results"]) {
  const root = $("results"); root.replaceChildren();
  if (!results.length) { const p=document.createElement("p");p.className="muted";p.textContent="完成的曲目会显示在这里。";root.append(p);return; }
  results.slice().reverse().forEach(item => {
    const row=document.createElement("div");row.className="result-row";
    const mark=document.createElement("span");mark.className="result-mark";mark.textContent="✓";
    const text=document.createElement("div");const strong=document.createElement("strong");strong.textContent=item.song || "已完成";
    const small=document.createElement("small");small.textContent=[item.artist,item.album].filter(Boolean).join(" · ") || item.path;
    text.append(strong,small);row.append(mark,text);root.append(row);
  });
}

function appendLog(payload: unknown) {
  const value = payload as {level?:string;message?:string}; if (!value?.message) return;
  const root=$("logs"); if (logCount===0) root.replaceChildren();
  const line=document.createElement("p");line.className=`log-line ${value.level || "info"}`;
  const time=document.createElement("time");time.textContent=new Date().toLocaleTimeString("zh-CN",{hour12:false});
  const text=document.createElement("span");text.textContent=value.message;line.append(time,text);root.append(line);logCount++;
  while(root.childElementCount>500) root.firstElementChild?.remove();root.scrollTop=root.scrollHeight;
}

function fillSettings(settings: Settings) {
  const values: Record<string,string|number> = {"project-root":settings.projectRoot,"storefront":settings.storefront,"alac-folder":settings.alacSaveFolder,"atmos-folder":settings.atmosSaveFolder,"aac-folder":settings.aacSaveFolder,"cover-size":settings.coverSize,"cover-format":settings.coverFormat,"lyrics-format":settings.lyricsFormat,"aac-type":settings.aacType,"alac-max":settings.alacMax,"atmos-max":settings.atmosMax};
  Object.entries(values).forEach(([id,value]) => { const input=$(id) as HTMLInputElement|HTMLSelectElement; if(document.activeElement!==input) input.value=String(value); });
  ($("embed-cover") as HTMLInputElement).checked=settings.embedCover; ($("embed-lyrics") as HTMLInputElement).checked=settings.embedLyrics; ($("save-lyrics") as HTMLInputElement).checked=settings.saveLyricsFile;
}

function readSettings(): Settings {
  if (!snapshot) throw new Error("设置尚未加载");
  return {...snapshot.settings, projectRoot:($("project-root") as HTMLInputElement).value.trim(),distribution:($("distro-select") as HTMLSelectElement).value,storefront:($("storefront") as HTMLInputElement).value.trim(),alacSaveFolder:($("alac-folder") as HTMLInputElement).value.trim(),atmosSaveFolder:($("atmos-folder") as HTMLInputElement).value.trim(),aacSaveFolder:($("aac-folder") as HTMLInputElement).value.trim(),coverSize:($("cover-size") as HTMLInputElement).value.trim(),coverFormat:($("cover-format") as HTMLSelectElement).value,lyricsFormat:($("lyrics-format") as HTMLSelectElement).value,aacType:($("aac-type") as HTMLSelectElement).value,alacMax:Number(($("alac-max") as HTMLSelectElement).value),atmosMax:Number(($("atmos-max") as HTMLSelectElement).value),embedCover:($("embed-cover") as HTMLInputElement).checked,embedLyrics:($("embed-lyrics") as HTMLInputElement).checked,saveLyricsFile:($("save-lyrics") as HTMLInputElement).checked};
}

async function refresh() { try { renderSnapshot(await invoke<Snapshot>("GetSnapshot")); } catch(error) { showToast(String(error),true); } }

document.querySelectorAll(".nav").forEach(el=>el.addEventListener("click",()=>setPage((el as HTMLElement).dataset.page!)));
document.querySelectorAll(".quality").forEach(el=>el.addEventListener("click",()=>{selectedQuality=(el as HTMLElement).dataset.quality!;document.querySelectorAll(".quality").forEach(q=>q.classList.toggle("active",q===el));}));
$("wrapper-chip").addEventListener("click",()=>setPage("wrapper"));
$("clear-log").addEventListener("click",()=>{logCount=0;$("logs").replaceChildren();});
$("refresh-wrapper").addEventListener("click",async()=>renderWrapper(await invoke<WrapperStatus>("RefreshWrapper")));
$("start-download").addEventListener("click",async()=>{try{const urls=($("url-input") as HTMLTextAreaElement).value.split(/\r?\n/);await invoke("StartDownloads",{urls,quality:selectedQuality});showToast("下载队列已启动");}catch(error){showToast(String(error),true);}});
$("cancel-download").addEventListener("click",()=>invoke("CancelDownloads"));
$("open-output").addEventListener("click",()=>invoke("OpenOutputFolder",selectedQuality).catch(error=>showToast(String(error),true)));
$("browse-project").addEventListener("click",async()=>{try{const path=await invoke<string>("BrowseProject");if(path)($("project-root") as HTMLInputElement).value=path;}catch(error){showToast(String(error),true);}});
$("save-settings").addEventListener("click",async()=>{try{const settings=await invoke<Settings>("SaveSettings",readSettings());if(snapshot)snapshot.settings=settings;fillSettings(settings);showToast("安全设置已保存");}catch(error){showToast(String(error),true);}});
$("start-wrapper").addEventListener("click",async()=>{const risk=$("risk-confirm") as HTMLInputElement;if(!risk.checked){showToast("请先确认本机进程可见性风险",true);return;}const id=$("apple-id") as HTMLInputElement;const password=$("apple-password") as HTMLInputElement;try{await invoke("StartWrapper",($("distro-select") as HTMLSelectElement).value,id.value,password.value);id.value="";password.value="";risk.checked=false;showToast("Wrapper 正在启动");}catch(error){password.value="";showToast(String(error),true);}});
$("submit-twofactor").addEventListener("click",async()=>{const input=$("twofactor-code") as HTMLInputElement;try{await invoke("SubmitTwoFactor",input.value);input.value="";showToast("验证码已提交");}catch(error){input.value="";showToast(String(error),true);}});
$("stop-wrapper").addEventListener("click",()=>invoke("StopWrapper").catch(error=>showToast(String(error),true)));
$("open-terminal").addEventListener("click",()=>invoke("OpenWrapperTerminal",($("distro-select") as HTMLSelectElement).value).catch(error=>showToast(String(error),true)));

window.runtime?.EventsOn("app:snapshot",data=>renderSnapshot(data as Snapshot));
window.runtime?.EventsOn("download:state",data=>renderDownload(data as DownloadState));
window.runtime?.EventsOn("wrapper:status",data=>renderWrapper(data as WrapperStatus));
window.runtime?.EventsOn("wrapper:twofactor",()=>setPage("wrapper"));
window.runtime?.EventsOn("app:log",appendLog);
void refresh();
window.setInterval(()=>invoke<WrapperStatus>("RefreshWrapper").then(status=>renderWrapper(status)).catch(()=>{}),5000);
