import type {MetadataRoute} from 'next';
import {categories} from './(site)/content';
export default function sitemap():MetadataRoute.Sitemap {
 if(process.env.DAPPER_PUBLIC_INDEXING!=='enabled')return [];
 return ['/','/services','/portfolio','/about-us','/contact-us','/studio',...categories.map(c=>'/'+c.slug)].map(path=>({url:'https://dapperdetailing.ca'+path}));
}
