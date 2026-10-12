// Purpose: actual-shaped store BFF method table fixture.
// Depends on: fetch and exact regex; no real server.
// Used by: CI-DRIFT source parser tests.
const uuid="[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}";
const routes={GET:new RegExp(`^widgets/${uuid}$`),POST:new RegExp(`^phantom$`)};
async function forward(request:Request,store:string,path:string){if(!routes[request.method]?.test(path))return new Response(null,{status:405});return fetch(`https://go.invalid/v1/admin/stores/${store}/${path}`,{method:request.method});}
export const GET=forward;export const POST=forward;
