# Existing generator routing adapter only: does not dispatch native operations.
import json,sys
from pathlib import Path
from operate import OUT
name='會員積分 v1 · CU2 CANDIDATE 20261107 · 6863a600'
create="""const page=await figma.getNodeByIdAsync('4:8829');await page.loadAsync();if(page.type!=='PAGE'||page.name!=='10 · SHOPLINE Redesign')throw Error('page identity');const formal=await figma.getNodeByIdAsync('1554:62573');if(formal.parent.id!==page.id||formal.children.length!==99)throw Error('formal inventory drift');if(page.children.some(c=>c.name===NAME))throw Error('candidate already exists: inspect, do not replay');await figma.setCurrentPageAsync(page);const section=figma.createSection();page.appendChild(section);section.name=NAME;section.x=530000;section.y=605000;section.resizeWithoutConstraints(35000,7000);return {operation:'CU2-create-6863a600',candidateId:section.id,parent:page.id,name:section.name,formalCount:formal.children.length};""".replace('NAME',json.dumps(name,ensure_ascii=False))
(OUT/'create-candidate.js').write_text(create)
if len(sys.argv)>1:
 cid=sys.argv[1]; assert cid!='1554:62573'
 config={'candidateId':cid,'candidateName':name,'pageId':'4:8829','formalId':'1554:62573'};(OUT/'candidate-target.json').write_text(json.dumps(config,ensure_ascii=False,indent=2))
 prefix='const CANDIDATE_ID='+json.dumps(cid)+';const CANDIDATE_NAME='+json.dumps(name,ensure_ascii=False)+';\n'
 lib=(OUT/'candidate-lib.js').read_text().replace("await PAGE.loadAsync();","await PAGE.loadAsync(); await figma.setCurrentPageAsync(PAGE);")
 screens=(OUT/'candidate-screens.js').read_text();keys=json.loads((OUT/'canonical-keys.json').read_text())
 for i in range(0,len(keys),4):
  subset=keys[i:i+4]; (OUT/f'generate-{i//4:02d}.js').write_text(prefix+'const KEYS='+json.dumps(subset)+';\n'+lib+'\n'+screens.replace('return res;', 'return {operationId:'+json.dumps('CU2-6863a600-batch-'+str(i//4))+',candidateId:SEC.id,keys:KEYS,frames:res};'))
 qa=Path('output/loyalty-points-ux/figma-src/qa.js').read_text();a=qa.index('const SEC=');b=qa.index('\nconst C=',a)
 qa=qa[:a]+"const SEC=await figma.getNodeByIdAsync(CANDIDATE_ID);assertCandidate(SEC,CANDIDATE_ID);"+qa[b:]
 (OUT/'qa-candidate.js').write_text(prefix+(OUT/'target-guard.js').read_text()+qa)
