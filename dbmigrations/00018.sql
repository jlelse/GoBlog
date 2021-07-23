update queue set schedule = toutc(schedule);
update persistent_cache set date = toutc(date);
update sessions set created = toutc(created), modified = toutc(modified), expires = toutc(expires);