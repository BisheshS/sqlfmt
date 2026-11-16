select dept, count(*) from emp group by dept having count(*) > 5 order by dept desc, count(*) limit 10 offset 20
