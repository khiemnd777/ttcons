import assert from 'node:assert/strict';
import test from 'node:test';
import { createRequire } from 'node:module';
import { fileURLToPath } from 'node:url';
import { build } from 'esbuild';
const require = createRequire(import.meta.url); const react = require('react');
const bundle = await build({stdin:{resolveDir:fileURLToPath(new URL('../src/',import.meta.url)),contents:`export {WorkflowFormPanel} from './modules/work-items/WorkflowFormPanel'; export {NativeExcelExportButton} from './modules/_shared/NativeExcelExportButton';`},bundle:true,write:false,platform:'node',format:'cjs',packages:'external',jsx:'automatic',loader:{'.css':'empty'},define:{'import.meta.env':'{}','import.meta.url':'"file:///fixture"'},plugins:[{name:'controlled-boundaries',setup(builder){
  builder.onLoad({filter:/\/lib\/api\/client\.ts$/},()=>({contents:`export class ApiError extends Error {}; export const downloadBlob=(path)=>globalThis.__formExcelDownload(path); export const get=async()=>({}); export const post=async()=>({}); export const put=post; export const del=post; export const postForm=post; export function setToken(){}; export function setAuthFailureHandler(){}; export function logoutRequest(){};`,loader:'js'}));
  builder.onLoad({filter:/AuthContext\.tsx$/},()=>({contents:`export const useAuth=()=>({user:{id:'actor'},hasPermission:()=>false});`,loader:'js'}));
  builder.onLoad({filter:/ToastProvider\.tsx$/},()=>({contents:`export const useToast=()=>({success(){},error(){},warn(){}});`,loader:'js'}));
}}]});
function fixture(name:string,data?:any){const state:any[]=[];let cursor=0;const hooks={...react,useState(initial:any){const at=cursor++;if(!(at in state))state[at]=typeof initial==='function'?initial():initial;return[state[at],(next:any)=>{state[at]=typeof next==='function'?next(state[at]):next;}];},useRef:(value:any)=>hooks.useState(()=>({current:value}))[0],useEffect(){},useImperativeHandle(){},useCallback:(fn:any)=>fn,useMemo:(fn:any)=>fn()};const module={exports:{} as any};new Function('require','module','exports',bundle.outputFiles[0].text)((id:string)=>id==='react'?hooks:id==='@tanstack/react-query'?{useQuery:(config:any)=>({data:config.queryKey[0]==='work-item-form'?data:[],isLoading:false,isError:false}),useQueryClient:()=>({invalidateQueries:async()=>{}}),useMutation:()=>({isPending:false,mutate(){},mutateAsync:async()=>{}})}:require(id),module,module.exports);return(props:any)=>{cursor=0;return module.exports[name](props);};}
function nodes(tree:any,matches:any[]=[]):any[]{if(!tree||typeof tree!=='object')return matches;matches.push(tree);for(const child of [tree.props?.children].flat(Infinity))nodes(child,matches);return matches;}
function actions(tree:any){return nodes(tree).filter(node=>node.type?.name==='NativeExcelExportButton');}
const data=(id:string='saved-instance',fields:string[]=['notes','blockers','next_plan_items'])=>({available:true,template:{code:'builtin_construction_daily_report',name:'Daily'},version:{version_no:3},instance:{id,status:'draft',form_type:'daily_report',current_revision:1},answers:{},fields:fields.map(field_key=>({id:field_key,field_key,label:field_key,field_type:'text',required:false,validation_json:{}})),capabilities:{}});
const props={workItemId:'work',projectId:'project',evidence:[]};

test('saved daily form toolbar exports exact instance as BM02, contextual blockers BM05 and unchanged next-plan BM06',async()=>{
  const tree=fixture('WorkflowFormPanel',data())(props);const buttons=actions(tree);
  assert.deepEqual(buttons.map(node=>node.props.filename),['BM-GS-02.xlsx','BM-GS-05.xlsx','BM-GS-06.xlsx']);
  assert.deepEqual(buttons.map(node=>node.props.label),['Xuất báo cáo ngày','Xuất báo cáo rủi ro','Xuất báo cáo hiện trường']);
  const paths:string[]=[];(globalThis as any).__formExcelDownload=async(path:string)=>{paths.push(path);throw new Error('bounded error');};
  for(const node of buttons){assert.equal(node.props.disabled,false);await fixture('NativeExcelExportButton')(node.props).props.onClick();}
  assert.deepEqual(paths,['/workflow-form-instances/saved-instance/export.xlsx','/workflow-form-instances/saved-instance/risks/export.xlsx','/workflow-form-instances/saved-instance/next-plan/export.xlsx']);
});

test('unsaved field edits disable all daily export requests, while another current saved identity remains exact',async()=>{
  const render=fixture('WorkflowFormPanel',data());
  const control=nodes(render(props)).find(node=>node.type?.name==='WorkflowFieldControl'&&node.props.field.field_key==='notes');assert.ok(control);control.props.onChange('Unsaved changes');
  const paths:string[]=[];(globalThis as any).__formExcelDownload=async(path:string)=>{paths.push(path);throw new Error('should not request');};
  for(const node of actions(render(props))){assert.equal(node.props.disabled,true);await fixture('NativeExcelExportButton')(node.props).props.onClick();}
  assert.deepEqual(paths,[]);
  assert.ok(actions(fixture('WorkflowFormPanel',data('other-current'))(props)).every(node=>node.props.path.startsWith('/workflow-form-instances/other-current/')));
});

test('no saved instance means no export; missing blockers schema hides contextual BM05 without removing BM02',()=>{
  assert.equal(actions(fixture('WorkflowFormPanel',{...data(),instance:undefined})(props)).length,0);
  assert.deepEqual(actions(fixture('WorkflowFormPanel',data('saved',['notes']))(props)).map(node=>node.props.filename),['BM-GS-02.xlsx']);
  for(const code of ['client_site_survey_minutes','client_survey_requirement_intake']){
    assert.equal(actions(fixture('WorkflowFormPanel',{...data(),template:{code}})(props)).length,0);
  }
});
