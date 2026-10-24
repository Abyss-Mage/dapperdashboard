import type {MetadataRoute} from 'next';
export default function robots():MetadataRoute.Robots {
 if(process.env.DAPPER_PUBLIC_INDEXING!=='enabled')return {rules:{userAgent:'*',disallow:'/'}};
 return {rules:{userAgent:'*',allow:'/',disallow:['/workspace','/owner','/content','/account','/booking','/enroll','/api/']},sitemap:'https://dapperdetailing.ca/sitemap.xml'};
}
