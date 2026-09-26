UPDATE tasks SET priority = CASE
    WHEN priority >= 9 THEN 5
    WHEN priority >= 7 THEN 3
    WHEN priority >= 5 THEN 2
    ELSE 1
END
WHERE (SELECT MAX(priority) FROM tasks) > 5;
