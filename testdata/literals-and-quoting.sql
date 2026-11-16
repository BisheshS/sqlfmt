select 'it''s from where' from t

select "Order", `from`, "a ""b""" from t

select 1, 2.5, .5, 1e10, 3.14E-2, -4, a - -4 from t

select a+b*2, c||d, x::int, a<>b, a>=1 from t

select t.order, T.End from t
