select u.id, o.total from users u left join orders o on u.id=o.user_id

select * from a join b on a.id=b.a_id inner join c on c.id=b.c_id and c.x=1

select * from a left outer join b on a.id=b.id

select left(name, 3) from t
