import type {MetadataRoute} from 'next';
import {publicPages} from '../../../../main website/pages.generated';
export default function sitemap():MetadataRoute.Sitemap {
 if(process.env.DAPPER_PUBLIC_INDEXING!=='enabled')return [];
 return ['/',...Object.keys(publicPages).map(slug=>'/'+slug+'/')].map(path=>({url:'https://dapperdetailing.ca'+path}));
}
