import {publicPages} from '../../../../../main website/pages.generated';
import {publicDocumentResponse} from '../_public/response';
export async function GET(request: Request, context: {params: Promise<{publicPage:string}>}) {
  const {publicPage} = await context.params;
  if (publicPage === 'home' || publicPage === 'my-bookings' || publicPage === 'book-appointment') {
    // Relative locations retain the browser's secure origin behind the gateway.
    return new Response(null,{status:publicPage === 'home'?308:307,headers:{Location:publicPage === 'home'?'/':publicPage === 'my-bookings'?'/account':'/booking'}});
  }
  if (!Object.hasOwn(publicPages,publicPage)) return new Response('Page not found',{status:404});
  return publicDocumentResponse(publicPages[publicPage]);
}
