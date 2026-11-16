select * from t where a=1 and b=2 or c=3

select * from t where age between 18 and 30 and active = true

select * from t where id in (1,2,3) and lower(name) = 'x'

select * from t where (a=1 or b=2) and c=3

select * from t where a is not null and b like 'x%'

select case when a > 1 and b < 2 then 'x' else 'y' end as r from t

select "select" as `from`, /* inline /* nested */ comment */ 1e-5+-2 as calc, 'it''s -- not a comment' as str from users where id in(1,2)and status!='active';--trailing comment

