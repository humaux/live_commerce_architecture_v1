// Purpose: bounded MOCK native-picker diagnostic; this is not Studio product or acceptance-test code.
// Depends on: installed Playwright Chromium and the Studio overlay's current DOM/CSS shape.
// Used by: PR2 round973 root-cause investigation; no accounts, DB, network, or action-sequence waits.
import { chromium } from '@playwright/test';
import { writeFile } from 'node:fs/promises';
const browser = await chromium.launch({headless:false});
const observations=[];
const traced=process.argv.includes("--trace");
try {
 for (const mode of ['explicit','prevent-default','native-only']) {
  for(let iteration=0;iteration<15;iteration++) {
   const context=await browser.newContext({viewport:{width:1586,height:992}});
   if(traced) await context.tracing.start({screenshots:true,snapshots:true});
   const page=await context.newPage();page.setDefaultTimeout(3000);
   await page.setContent(`<style>.studio-schedule-picker{position:relative;display:grid;place-items:center;width:44px;height:44px;border:1px solid;border-radius:5px}.studio-schedule-picker input{position:absolute;inset:0;width:100%;height:100%;opacity:0;cursor:pointer}</style><span class="studio-schedule-picker"><input aria-label="picker" type="datetime-local" value="2030-01-01T08:00" min="2000-01-01T00:00" max="2199-12-31T23:59"></span><script>
   window.events=[];const input=document.querySelector('input');
   for(const name of ['focus','blur','keydown','beforeinput','input','change']) input.addEventListener(name,event=>events.push({event:name,trusted:event.isTrusted,key:event.key,value:input.value,at:performance.now()}));
   input.addEventListener('click',event=>{events.push({event:'click',trusted:event.isTrusted,at:performance.now()});if('${mode}'==='native-only')return;if('${mode}'==='prevent-default')event.preventDefault();try{input.showPicker();events.push({event:'showPicker',at:performance.now()});}catch(error){events.push({event:'showPicker-error',name:error.name});}});
   </script>`);
   await page.bringToFront();
   const picker=page.getByLabel('picker');
   await picker.click();
   await page.keyboard.press('ArrowRight');
   await page.keyboard.press('Enter');
   const picked=await picker.inputValue();
   const events=await page.evaluate(()=>window.events); // READ/MEASURE after the unchanged action sequence.
   observations.push({mode,iteration,picked,changed:picked!=='2030-01-01T08:00',events});
   if(traced) await context.tracing.stop();
   await context.close();
  }
 }
}finally{await browser.close();}
const result={platform:process.platform,arch:process.arch,traced,observations};
await writeFile(`/Volumes/data/live_commerce_architecture_v1/output/w3-u4-parcel-merge-ui/round973/native-picker-${traced?'traced':'plain'}-probe.json`,JSON.stringify(result,null,2));
console.log(JSON.stringify(['explicit','prevent-default','native-only'].map(mode=>({mode,total:observations.filter(v=>v.mode===mode).length,changed:observations.filter(v=>v.mode===mode&&v.changed).length}))));
