import {homeHTML} from '../../../../main website/pages.generated';
import {publicDocumentResponse} from './_public/response';
export const dynamic = 'force-static';
// A complete document keeps cinematic CSS/scripts separate from operational UI.
export function GET() {
  return publicDocumentResponse(homeHTML);
}
