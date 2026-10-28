import {publicPages} from '../../../../../main website/pages.generated';
import {publicDocumentResponse} from '../_public/response';
export const dynamic = 'force-static';
// The interactive Porsche studio is still a complete static document of its own.
export function GET() {
  return publicDocumentResponse(publicPages.studio);
}
