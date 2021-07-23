drop trigger AFTER;
create trigger trigger_posts_delete_pp after delete on posts begin delete from post_parameters where path = old.path; end;