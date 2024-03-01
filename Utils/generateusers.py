## Chatgpt generated 
import psycopg2
from faker import Faker
import random

# Connect to your PostgreSQL database
conn = psycopg2.connect(
    host="localhost",
    database="tickets",
    user="rayanc",
    password="rayanc"
)

# Create a cursor object to execute SQL queries
cursor = conn.cursor()

# Function to generate random data and insert into the Users table
def generate_random_data(num_records):
    fake = Faker()

    for _ in range(num_records):
        username = fake.user_name()
        password = fake.password()

        # Execute the SQL query to insert data into the Users table
        cursor.execute("INSERT INTO Users (username, password) VALUES (%s, %s)", (username, password))

    # Commit the changes to the database
    conn.commit()

# Specify the number of random records to generate
num_records_to_generate = 10

# Call the function to generate and insert
