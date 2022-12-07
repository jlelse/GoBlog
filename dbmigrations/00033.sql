alter table activitypub_followers add username text not null default "";
update activitypub_followers set username = follower;