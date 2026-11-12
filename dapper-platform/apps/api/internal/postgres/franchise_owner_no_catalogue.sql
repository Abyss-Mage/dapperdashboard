-- Services are decided centrally: a franchise owner manages the slots of their services, not the services.
DELETE FROM role_capabilities rc USING roles r WHERE rc.role_id=r.id AND lower(r.name)='franchise owner' AND rc.action='catalogue.manage';
