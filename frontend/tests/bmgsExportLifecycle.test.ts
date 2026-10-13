import assert from 'node:assert/strict';
import test from 'node:test';
import { build } from 'esbuild';
import { createRequire } from 'node:module';
import { fileURLToPath } from 'node:url';
const require = createRequire(import.meta.url);
const react = require('react');
const bundle = await build({stdin: {resolveDir: fileURLToPath(new URL('../src/', import.meta.url)), contents: `export {WorkReportReviewDialog} from './modules/work-reports/WorkReportReviewDialog'; export {WorkReportsPage} from './modules/work-reports/WorkReportsPage'; export {OperationsPage} from './modules/operations/OperationsPage'; export {ConstructionReportDetailDialog} from './modules/operations/ConstructionReportDetailDialog';`}, bundle:true, write:false, platform:'node', format:'cjs', packages:'external', jsx:'automatic', loader:{'.css':'empty'}, define:{'import.meta.env':'{}','import.meta.url':'"file:///fixture"'}, plugins:[{name:'boundaries',setup(builder){
  builder.onLoad({filter:/\/lib\/api\/client\.ts$/},()=>({contents:`export class ApiError extends Error {}; export const downloadBlob=(path)=>globalThis.__nativeDownload(path); export const get=async()=>[]; export const post=async()=>({}); export const put=post; export const del=post; export const postFormWithProgress=post; export const postDownloadBlob=post; export function setAuthFailureHandler(){}; export function setToken(){}; export function logoutRequest(){};`,loader:'js'}));
  builder.onLoad({filter:/\/ToastProvider\.tsx$/},()=>({contents:`export const useToast=()=>globalThis.__nativeToast;`,loader:'js'}));
  builder.onLoad({filter:/\/AuthContext\.tsx$/},()=>({contents:`export const useAuth=()=>({user:{id:'actor'},hasPermission:()=>globalThis.__nativeAllowed,hasAnyPermission:()=>globalThis.__nativeAllowed});`,loader:'js'}));
}}]});
function fixture(component: string, queries: any = {}) {
  const states:any[]=[]; let cursor=0;
  const hooks={...react,useState(initial:any){const index=cursor++; if(!(index in states)) states[index]=typeof initial==='function'?initial():initial; return [states[index],(next:any)=>{states[index]=typeof next==='function'?next(states[index]):next;}];},useEffect(){},useMemo:(fn:any)=>fn(),useCallback:(fn:any)=>fn,useRef:(value:any)=>({current:value})};
  const module={exports:{} as any};
  new Function('require','module','exports',bundle.outputFiles[0].text)((name:string)=>name==='react'?hooks:name==='react-router-dom'?{...require(name),useNavigate:()=>()=>{},useParams:()=>({id:'project'}),useLocation:()=>({pathname:'/operations'}),useSearchParams:()=>[new URLSearchParams(),()=>{}]}:name==='@tanstack/react-query'?{...require(name),useQuery:(config:any)=>({data:queries[config.queryKey[0]]??[],isLoading:false,isFetching:false,isError:false,refetch:async()=>{},isSuccess:true}),useQueryClient:()=>({invalidateQueries:async()=>{}}),useMutation:()=>({mutateAsync:async()=>{},isPending:false})}:require(name),module,module.exports);
  return (props:any={})=>{cursor=0; return module.exports[component](props);};
}
function nodes(tree:any,matches:any[]=[]):any[]{if(!tree||typeof tree!=='object')return matches; matches.push(tree); for(const child of [tree.props?.children,tree.props?.footer,tree.props?.actions].flat(Infinity))nodes(child,matches);return matches;}
function text(tree:any):string{return typeof tree==='string'?tree:tree&&typeof tree==='object'?[tree.props?.children].flat(Infinity).map(text).join(''):'';}
function button(tree:any,label:string){return nodes(tree).find(node=>node.type==='button'&&text(node).includes(label));}
const globals=globalThis as any;
const toastEvents:string[]=[];
globals.__nativeToast={success:(value:string)=>toastEvents.push(value),error:(value:string)=>toastEvents.push(value),warn() {}};
globals.__nativeAllowed=true;
const detail={work_report_id:'formal',source_id:'daily',current_revision_no:4,project_id:'project',report_date:'2026-10-08',status:'submitted',revisions:[{revision_no:3,snapshot_json:{},evidence_records:[]},{revision_no:4,snapshot_json:{},evidence_records:[]}],events:[],read_receipts:[]};
const props={open:true,detail,loading:false,error:null,userID:'actor',canReview:false,canManage:false,canOpenSource:false,canExport:true,onRefresh:async()=>{},onClose(){}};

test('historical native export clicks exact selected revision, shows pending, handles error and retries without source writes',async()=>{
  const render=fixture('WorkReportReviewDialog'); let reject:any; const paths:string[]=[];
  globals.__nativeDownload=(path:string)=>{paths.push(path);return new Promise((_resolve,rejectPromise)=>{reject=rejectPromise;});};
  const request=button(render({...props,initialRevisionNo:3}),'Xuất báo cáo').props.onClick();
  assert.equal(paths[0],'/work-reports/formal/revisions/3/export.xlsx');
  assert.equal(button(render({...props,initialRevisionNo:3}),'Đang xuất').props.disabled,true);
  reject(new Error('Missing report information')); await new Promise(resolve=>setImmediate(resolve)); await request;
  assert.equal(button(render({...props,initialRevisionNo:3}),'Xuất báo cáo').props.disabled,false);
  assert.ok(toastEvents.includes('Không xuất được Excel báo cáo'));
  button(render({...props,initialRevisionNo:3}),'Xuất báo cáo').props.onClick(); assert.equal(paths.length,2);reject(new Error('Still invalid'));await new Promise(resolve=>setImmediate(resolve));
});

test('native revision export unavailable for unauthorized, missing revision, loading and errors',()=>{
  for(const change of [{canExport:false},{initialRevisionNo:9},{loading:true},{error:new Error('Unavailable')}])assert.equal(button(fixture('WorkReportReviewDialog')({...props,...change}),'Xuất báo cáo'),undefined);
});

test('current saved locked report exports its native identity and permission denial hides action',async()=>{
  globals.__nativeDownload=async(path:string)=>{assert.equal(path,'/daily-reports/daily/export.xlsx');throw new Error('bounded error');};
  const render=fixture('ConstructionReportDetailDialog');
  const current={open:true,projectId:'project',record:{id:'daily',delivery_status:'submitted'},canManage:false,onClose(){}};
  button(render(current),'Xuất báo cáo ngày').props.onClick(); await new Promise(resolve=>setImmediate(resolve));
  globals.__nativeAllowed=false; assert.equal(button(render(current),'Xuất báo cáo ngày'),undefined);globals.__nativeAllowed=true;
});

test('work report list exports applied scope and separates register export, with empty list disabled',async()=>{
  const paths:string[]=[];globals.__nativeDownload=async(path:string)=>{paths.push(path);throw new Error('bounded error');};
  const render=fixture('WorkReportsPage',{'work-reports':[{id:'report'}]});
  const tree=render();const register=nodes(tree).find(node=>node.type?.name==='ExportControl');assert.ok(register);assert.ok(button(register.type(register.props),'Xuất sổ đăng ký'));
  button(tree,'Xuất báo cáo').props.onClick();await new Promise(resolve=>setImmediate(resolve));
  assert.equal(paths[0],'/work-reports/export.xlsx?scope=mine');
  assert.equal(button(fixture('WorkReportsPage')(),'Xuất báo cáo').props.disabled,true);
});

test('operations list applies dates explicitly; export ignores uncommitted date inputs',async()=>{
  const paths:string[]=[];globals.__nativeDownload=async(path:string)=>{paths.push(path);throw new Error('bounded error');};
  const render=fixture('OperationsPage',{'daily-reports':[{id:'daily'}],'construction-workspace':{runtime:{construction_active:true},checks:[],actions:[]},'construction-variation-context':{variations:[]}});
  const dateInputs=nodes(render()).filter(node=>node.type==='input'&&node.props.type==='date');
  dateInputs[0].props.onChange({target:{value:'2026-10-01'}});dateInputs[1].props.onChange({target:{value:'2026-10-08'}});
  button(render(),'Xuất báo cáo').props.onClick();await new Promise(resolve=>setImmediate(resolve));
  assert.equal(paths[0],'/projects/project/daily-reports/export.xlsx');
  button(render(),'Áp dụng').props.onClick();button(render(),'Xuất báo cáo').props.onClick();await new Promise(resolve=>setImmediate(resolve));
  assert.equal(paths[1],'/projects/project/daily-reports/export.xlsx?date_from=2026-10-01&date_to=2026-10-08');
});
