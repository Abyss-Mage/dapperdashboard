import type {MetadataRoute} from 'next';
export default function robots():MetadataRoute.Robots {
 if(process.env.DAPPER_PUBLIC_INDEXING!=='enabled')return {rules:{userAgent:'*',disallow:'/'}};
 return {rules:{userAgent:'*',allow:'/',disallow:['/workspace','/dashboard','/owner','/content','/account','/book-appointment','/enroll','/pay/','/shop','/dealer','/api/']},sitemap:'https://dapperdetailing.ca/sitemap.xml'};
}
